// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package main

// The daemon maintains a pf anchor that lets the protected app's group (default: _netleash) reach the network ONLY
// through the VPN's tunnel interface (and loopback).
//
// Fail-closed by construction: the "pass on utunN" rule stops matching the moment that interface disappears, so
// the kernel blocks traffic instantly, with or without this daemon. The daemon exists to
//   (a) point the rule at the new utunN after a reconnect,
//   (b) restore the pf anchor reference if somebody resets pf,
//   (c) kill protected-app processes that were started outside the launcher,
//   (d) publish status for the menu bar app and the CLI.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sedooj/netleash/internal/config"
	"github.com/sedooj/netleash/internal/netinfo"
	"github.com/sedooj/netleash/internal/proc"
	"github.com/sedooj/netleash/internal/rules"
	"github.com/sedooj/netleash/internal/sys"
)

type daemon struct {
	cfg config.Config

	gid atomic.Uint32 // gid of the confined group, 0 = unknown

	// state owned by the slow loop (tick)
	learned     map[string]bool
	lastRules   string
	subnets     []netip.Prefix // attached physical subnets (cache)
	subnetsAt   time.Time
	iface       string // effective VPN interface ("" = down)
	nilStreak   int
	tickCount   int
	learnedFor  string
	lastIface   string
	started     bool // lastIface is valid
	lookupDelay int

	mu              sync.Mutex // guards lastRogueNotify and event
	lastRogueNotify time.Time
	event           *event // last user-facing event, published in the status file for the menu bar app
}

// event is a user-facing notification. The root daemon cannot post notifications under the Netleash name itself,
// so it publishes the event in the status file and the menu bar app (running as the user) shows it.
type event struct {
	Seq   int64  `json:"seq"`
	Kind  string `json:"kind"` // vpn_down, vpn_up, app_closed: the menu bar app shows localized text for known kinds
	Title string `json:"title"`
	Text  string `json:"text"`
}

func runDaemon() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "netleash daemon must run as root")
		os.Exit(1)
	}
	log.SetFlags(0)
	d := &daemon{cfg: config.Load(config.ConfigPath), learned: loadLearned()}
	d.logf("started (version %s); group %s, protected app %s, VPN apps %s (detection: %s)",
		version, d.cfg.GroupName, d.cfg.ProtectedAppPath, strings.Join(d.cfg.VPNApps(), ", "), d.cfg.VPNDetection)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigs
		d.logf("signal received - the pf anchor stays in the kernel (fail-closed)")
		os.Exit(0)
	}()

	go func() { // fast loop: unprotected launches of the protected app must die within ~50 ms
		for range time.Tick(50 * time.Millisecond) {
			d.scanRogue()
		}
	}()
	d.tick()
	for range time.Tick(500 * time.Millisecond) {
		d.tick()
	}
}

func (d *daemon) logf(format string, a ...any) {
	log.Printf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}

// ---- group ----

// lookupGID resolves the confined group's gid through Directory Services (macOS groups are not in /etc/group).
func (d *daemon) lookupGID() uint32 {
	out, ok := sys.Run("/usr/bin/dscl", ".", "-read", "/Groups/"+d.cfg.GroupName, "PrimaryGroupID")
	if !ok {
		return 0
	}
	f := strings.Fields(out)
	if len(f) == 2 && f[0] == "PrimaryGroupID:" {
		if n, err := strconv.ParseUint(f[1], 10, 32); err == nil {
			return uint32(n)
		}
	}
	return 0
}

// ---- VPN detection ----

// detectVPNInterface returns the tunnel interface to allow, or "".
//
// "app" mode (default): a process of one of the VPN apps must be running; utunN names on its command line are tried
// first (tun2socks), otherwise the tunnel that carries the default route is taken (WireGuard/AmneziaWG ask the kernel
// for "utun" and the number is chosen later, and a NetworkExtension tunnel is created by the system).
// "any" mode: no app is required, any tunnel carrying the default route is taken.
func (d *daemon) detectVPNInterface(procs []proc.Proc) string {
	var explicit []string
	seen := map[string]bool{}
	running := false
	for _, p := range procs {
		for _, app := range d.cfg.VPNApps() {
			if !strings.HasPrefix(p.Path, app+"/") {
				continue
			}
			running = true
			_, args := proc.Args(p.PID)
			for _, n := range netinfo.UtunNames(strings.Join(args, " ")) {
				if !seen[n] {
					seen[n] = true
					explicit = append(explicit, n)
				}
			}
		}
	}
	if !running && d.cfg.VPNDetection != "any" {
		return ""
	}
	sort.Strings(explicit)
	list, _ := sys.Run("/sbin/ifconfig", "-l")
	routes, _ := sys.Run("/usr/sbin/netstat", "-rn", "-f", "inet")
	return netinfo.PickTunnel(explicit, netinfo.UtunInterfaces(list), routes, func(n string) string {
		out, _ := sys.Run("/sbin/ifconfig", n)
		return out
	})
}

// ---- VPN DNS auto-detection ----

func loadLearned() map[string]bool {
	m := map[string]bool{}
	if data, err := os.ReadFile(config.LearnedDNSPath); err == nil {
		var list []string
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list {
				m[s] = true
			}
		}
	}
	return m
}

func physicalInterfaces() []string {
	out, _ := sys.Run("/sbin/ifconfig", "-l")
	return netinfo.PhysicalInterfaces(out)
}

// learnVPNDNS finds the VPN's DNS servers: addresses from `scutil --dns` that have a host route via the VPN interface.
func (d *daemon) learnVPNDNS(iface string) {
	if !d.cfg.AutoDetectVPNDNS {
		return
	}
	scutil, _ := sys.Run("/usr/sbin/scutil", "--dns")
	routes, _ := sys.Run("/usr/sbin/netstat", "-rn", "-f", "inet")
	var subnets []netipPrefix
	for _, i := range physicalInterfaces() {
		out, _ := sys.Run("/sbin/ifconfig", i)
		subnets = append(subnets, netinfo.Subnets(out)...)
	}
	found := netinfo.FilterVPNDNS(netinfo.Nameservers(scutil), netinfo.HostRoutesVia(routes, iface), subnets)
	var added []string
	for _, s := range found {
		if !d.learned[s] {
			d.learned[s] = true
			added = append(added, s)
		}
	}
	if len(added) == 0 {
		return
	}
	var all []string
	for s := range d.learned {
		all = append(all, s)
	}
	sort.Strings(all)
	if data, err := json.Marshal(all); err == nil {
		_ = os.WriteFile(config.LearnedDNSPath, data, 0o644)
	}
	d.logf("detected VPN DNS servers: %s", strings.Join(added, ", "))
}

// attachedSubnets returns the subnets directly attached to physical interfaces (cached for a few seconds: it costs one
// ifconfig per interface).
func (d *daemon) attachedSubnets() []netip.Prefix {
	if time.Since(d.subnetsAt) < 5*time.Second {
		return d.subnets
	}
	var subnets []netip.Prefix
	for _, i := range physicalInterfaces() {
		out, _ := sys.Run("/sbin/ifconfig", i)
		subnets = append(subnets, netinfo.Subnets(out)...)
	}
	d.subnets, d.subnetsAt = subnets, time.Now()
	return subnets
}

// vpnDNSBlockList is the VPN's DNS servers that must not be reachable through physical interfaces. An address that lies
// inside a subnet of the network we are on now (a carrier network whose gateway is 100.64.0.1, say) is that network's
// own server and must keep working, so it is left out.
func (d *daemon) vpnDNSBlockList() []string {
	set := map[string]bool{}
	for _, s := range d.cfg.VPNDNSServers {
		set[s] = true
	}
	for s := range d.learned {
		set[s] = true
	}
	attached := d.attachedSubnets()
	var out []string
	for s := range set {
		if _, err := parseAddr(s); err == nil && !netinfo.OnLink(s, attached) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// ---- pf ----

// ensureMainRuleset makes sure pf is enabled and the main ruleset contains `anchor "com.apple/*"`, through
// which our anchor runs.
func (d *daemon) ensureMainRuleset() bool {
	info, _ := sys.Run("/sbin/pfctl", "-si")
	main, _ := sys.Run("/sbin/pfctl", "-sr")
	enabled := strings.Contains(info, "Status: Enabled")
	hooked := false
	for _, l := range strings.Split(main, "\n") {
		if strings.HasPrefix(l, `anchor "com.apple/*"`) {
			hooked = true
		}
	}
	if enabled && hooked {
		return true
	}
	d.logf("pf was changed externally (enabled: %v, anchor com.apple/*: %v); main ruleset:\n%s", enabled, hooked, main)
	// Restore Apple's stock ruleset, which contains anchor "com.apple/*".
	if out, ok := sys.Run("/sbin/pfctl", "-f", "/etc/pf.conf"); !ok {
		d.logf("pfctl -f failed: %s", out)
		return false
	}
	if !enabled {
		out, _ := sys.Run("/sbin/pfctl", "-E")
		if i := strings.Index(out, "Token : "); i >= 0 {
			if f := strings.Fields(out[i+len("Token : "):]); len(f) > 0 {
				_ = os.WriteFile(config.PFTokenPath, []byte(f[0]), 0o600)
			}
		}
	}
	d.lastRules = ""
	d.logf("main pf ruleset restored")
	return true
}

func (d *daemon) applyAnchor(iface string, dnsLock bool) bool {
	text := rules.Build(rules.Params{
		Group: d.cfg.GroupName, VPNIface: iface, PhysicalIfaces: physicalInterfaces(),
		VPNDNS: d.vpnDNSBlockList(), DNSLock: dnsLock, LoopbackBlockPorts: d.cfg.LoopbackBlockPorts,
	})
	if text == d.lastRules {
		// Verify that the kernel really holds our rules.
		if cur, _ := sys.Run("/sbin/pfctl", "-a", config.AnchorName, "-sr"); strings.Contains(cur, "block drop quick proto") {
			return true
		}
	}
	if out, ok := sys.RunStdin(text, "/sbin/pfctl", "-a", config.AnchorName, "-f", "-"); !ok {
		d.logf("failed to load anchor: %s", out)
		return false
	}
	d.lastRules = text
	return true
}

// ---- processes ----

func (d *daemon) inProtectedApp(p proc.Proc) bool {
	return strings.HasPrefix(p.Path, d.cfg.ProtectedAppPath+"/")
}

// scanRogue kills every process of the protected app that is outside the confined group.
func (d *daemon) scanRogue() {
	gid := d.gid.Load()
	if !d.cfg.EnforceLauncherOnly || gid == 0 {
		return
	}
	procs, err := proc.List()
	if err != nil {
		return
	}
	var rogue []proc.Proc
	for _, p := range procs {
		if !d.inProtectedApp(p) || p.GID == gid {
			continue
		}
		ignored := false
		for _, s := range d.cfg.IgnoreCommandSubstrings {
			if strings.Contains(p.Path, s) {
				ignored = true
			}
		}
		if !ignored {
			rogue = append(rogue, p)
		}
	}
	if len(rogue) == 0 {
		return
	}
	killAll(rogue)
	d.logf("unprotected launch of the protected app (%d processes) terminated", len(rogue))
	d.mu.Lock()
	notifyNow := time.Since(d.lastRogueNotify) > time.Minute
	if notifyNow {
		d.lastRogueNotify = time.Now()
	}
	d.mu.Unlock()
	if notifyNow {
		d.notify("app_closed", "App closed", "Start it only through Netleash.")
	}
}

func killAll(ps []proc.Proc) {
	for _, p := range ps {
		_ = syscall.Kill(p.PID, syscall.SIGKILL)
	}
}

// ---- notifications and status ----

func (d *daemon) notify(kind, title, text string) {
	if !d.cfg.Notify {
		return
	}
	d.mu.Lock()
	d.event = &event{Seq: time.Now().UnixMilli(), Kind: kind, Title: title, Text: text}
	d.mu.Unlock()
	if !d.menuAppRunning() {
		d.notifyFallback(title, text)
	}
}

func (d *daemon) menuAppRunning() bool {
	procs, err := proc.List()
	if err != nil {
		return false
	}
	for _, p := range procs {
		if strings.Contains(p.Path, "/Netleash.app/Contents/MacOS/") {
			return true
		}
	}
	return false
}

// notifyFallback is used only when the menu bar app is not running (it would be silent otherwise). macOS shows such
// notifications under the generic "Script Editor" name, which is why the app path is preferred.
func (d *daemon) notifyFallback(title, text string) {
	fi, err := os.Stat("/dev/console")
	if err != nil {
		return
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Uid == 0 {
		return
	}
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, esc.Replace(text), esc.Replace(title))
	go sys.Run("/bin/launchctl", "asuser", strconv.Itoa(int(st.Uid)), "/usr/bin/osascript", "-e", script)
}

// ---- status ----

func (d *daemon) writeStatus(armed bool, iface, note string) {
	_ = os.MkdirAll(config.StateDir, 0o755)
	d.mu.Lock()
	ev := d.event
	d.mu.Unlock()
	data, err := json.Marshal(map[string]any{
		"armed": armed, "vpnUp": iface != "", "iface": iface, "ts": float64(time.Now().UnixNano()) / 1e9,
		"pid": os.Getpid(), "gid": d.gid.Load(), "note": note, "event": ev,
	})
	if err != nil {
		return
	}
	tmp := filepath.Join(config.StateDir, ".status.tmp")
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Chmod(tmp, 0o644)
		_ = os.Rename(tmp, config.StatusPath)
	}
}

// ---- main tick ----

func (d *daemon) tick() {
	d.tickCount++
	if d.gid.Load() == 0 || d.tickCount%20 == 0 {
		d.gid.Store(d.lookupGID())
	}
	gid := d.gid.Load()
	if gid == 0 {
		d.writeStatus(false, "", fmt.Sprintf("group %s is missing", d.cfg.GroupName))
		d.logf("group %s not found", d.cfg.GroupName)
		return
	}
	procs, err := proc.List()
	if err != nil {
		d.logf("cannot list processes: %v", err)
		return
	}

	// A single failed detection must not flap rules and notifications: "no VPN" is accepted after 3 consecutive
	// misses (1.5 s). A real disconnect is blocked by the kernel anyway.
	if raw := d.detectVPNInterface(procs); raw != "" {
		d.nilStreak, d.iface = 0, raw
	} else if d.nilStreak++; d.nilStreak >= 3 {
		d.iface = ""
	}
	iface := d.iface
	if iface != "" && (iface != d.learnedFor || d.tickCount%20 == 0) {
		d.learnVPNDNS(iface)
		d.learnedFor = iface
	}

	mainOK := d.ensureMainRuleset()
	appRunning := false
	for _, p := range procs {
		if p.GID == gid && d.inProtectedApp(p) {
			appRunning = true
			break
		}
	}
	armed := mainOK && d.applyAnchor(iface, iface == "" && appRunning)

	if !armed {
		// We cannot guarantee blocking: stop everything that is in the confined group.
		var victims []proc.Proc
		for _, p := range procs {
			if p.GID == gid && p.PID != os.Getpid() {
				victims = append(victims, p)
			}
		}
		if len(victims) > 0 {
			d.logf("protection is not armed, killing %d processes", len(victims))
			killAll(victims)
		}
		d.writeStatus(false, iface, "pf is not armed")
	} else if iface == "" {
		d.writeStatus(true, "", "VPN is down: network of the protected app is blocked")
	} else {
		d.writeStatus(true, iface, "VPN "+iface+": network of the protected app is allowed only through it")
	}

	if !d.started || d.lastIface != iface {
		switch {
		case d.started && iface == "":
			d.logf("VPN down -> protected app blocked")
			if appRunning {
				d.notify("vpn_down", "VPN disconnected", "Network access is blocked.")
			}
		case d.started:
			d.logf("VPN up (%s) -> protected app allowed", iface)
			if appRunning {
				d.notify("vpn_up", "VPN connected", "Network access restored through the VPN.")
			}
		case iface == "":
			d.logf("VPN is not connected")
		default:
			d.logf("VPN: %s", iface)
		}
		d.lastIface, d.started = iface, true
	}
}
