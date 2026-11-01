// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package netinfo parses the output of macOS network tools. It contains no system calls, so it is fully testable.
package netinfo

import (
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	utunRe       = regexp.MustCompile(`utun[0-9]+`)
	nameserverRe = regexp.MustCompile(`nameserver\[[0-9]+\] : ([0-9.]+)`)
	physIfaceRe  = regexp.MustCompile(`^en[0-9]+$`)
	utunExactRe  = regexp.MustCompile(`^utun[0-9]+$`)

	cgnat     = netip.MustParsePrefix("100.64.0.0/10")
	linkLocal = netip.MustParsePrefix("169.254.0.0/16")
)

// UtunNames returns the distinct utunN names mentioned in text.
func UtunNames(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range utunRe.FindAllString(text, -1) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// InterfaceIsUp reports whether `ifconfig <iface>` output describes an up, running interface with an IPv4 address.
func InterfaceIsUp(ifconfig string) bool {
	return strings.Contains(ifconfig, "<UP") && strings.Contains(ifconfig, "RUNNING") && strings.Contains(ifconfig, "inet ")
}

// route is one parsed line of `netstat -rn -f inet`.
type route struct{ dest, flags, iface string }

func parseRoutes(netstat string) []route {
	var rs []route
	for _, line := range strings.Split(netstat, "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 {
			rs = append(rs, route{dest: f[0], flags: f[2], iface: f[3]})
		}
	}
	return rs
}

// HasDefaultRoute reports whether iface carries a default-like route (default, 0/1 or 128.0/1) that is not
// interface-scoped (flag I).
func HasDefaultRoute(netstat, iface string) bool {
	for _, r := range parseRoutes(netstat) {
		if r.iface != iface || strings.Contains(r.flags, "I") {
			continue
		}
		switch r.dest {
		case "default", "0/1", "128.0/1":
			return true
		}
	}
	return false
}

// HostRoutesVia returns destinations that are full IPv4 addresses routed through iface.
func HostRoutesVia(netstat, iface string) map[string]bool {
	out := map[string]bool{}
	for _, r := range parseRoutes(netstat) {
		if r.iface == iface {
			if a, err := netip.ParseAddr(r.dest); err == nil && a.Is4() {
				out[r.dest] = true
			}
		}
	}
	return out
}

// Nameservers returns the IPv4 nameservers listed in `scutil --dns` output.
func Nameservers(scutil string) map[string]bool {
	out := map[string]bool{}
	for _, m := range nameserverRe.FindAllStringSubmatch(scutil, -1) {
		if a, err := netip.ParseAddr(m[1]); err == nil && a.Is4() {
			out[m[1]] = true
		}
	}
	return out
}

// IsPrivate reports whether a is RFC1918, CGNAT (100.64/10) or link-local. Only such VPN DNS servers are blocked
// automatically; a public one such as 8.8.8.8 may have been configured by the user for normal use.
func IsPrivate(a netip.Addr) bool {
	return a.Is4() && (a.IsPrivate() || cgnat.Contains(a) || linkLocal.Contains(a))
}

// Subnets returns the IPv4 subnets configured in `ifconfig <iface>` output ("inet A netmask 0xHEX").
func Subnets(ifconfig string) []netip.Prefix {
	var out []netip.Prefix
	for _, line := range strings.Split(ifconfig, "\n") {
		f := strings.Fields(line)
		for i := 0; i+3 < len(f); i++ {
			if f[i] != "inet" || f[i+2] != "netmask" || !strings.HasPrefix(f[i+3], "0x") {
				continue
			}
			addr, err := netip.ParseAddr(f[i+1])
			mask, err2 := strconv.ParseUint(f[i+3][2:], 16, 32)
			if err != nil || err2 != nil || !addr.Is4() {
				continue
			}
			ones := 0
			for m := uint32(mask); m&0x80000000 != 0; m <<= 1 {
				ones++
			}
			if p, err := addr.Prefix(ones); err == nil {
				out = append(out, p)
			}
		}
	}
	return out
}

// FilterVPNDNS picks the nameservers that are routed through the VPN, are private, and do not belong to a subnet
// directly attached to a physical interface (a router's DNS there must stay reachable).
func FilterVPNDNS(servers, viaVPN map[string]bool, physical []netip.Prefix) []string {
	var out []string
	for s := range servers {
		if !viaVPN[s] {
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil || !IsPrivate(a) {
			continue
		}
		attached := false
		for _, p := range physical {
			if p.Contains(a) {
				attached = true
				break
			}
		}
		if !attached {
			out = append(out, s)
		}
	}
	return out
}

// IsPhysicalInterface reports whether name looks like a physical interface (en0, en1, …).
func IsPhysicalInterface(name string) bool { return physIfaceRe.MatchString(name) }

// PhysicalInterfaces filters the output of `ifconfig -l`.
func PhysicalInterfaces(ifconfigList string) []string {
	var out []string
	for _, n := range strings.Fields(ifconfigList) {
		if IsPhysicalInterface(n) {
			out = append(out, n)
		}
	}
	return out
}

// PickTunnel chooses the tunnel interface to allow. explicit are utunN names found on the VPN app's command lines (tried
// first); all are every utunN on the system. A tunnel qualifies when it is up, has an IPv4 address and carries a
// default-like route. ifconfigOf returns `ifconfig <name>` output. Returns "" when nothing qualifies.
func PickTunnel(explicit, all []string, netstat string, ifconfigOf func(string) string) string {
	ok := func(name string) bool { return InterfaceIsUp(ifconfigOf(name)) && HasDefaultRoute(netstat, name) }
	for _, c := range explicit {
		if ok(c) {
			return c
		}
	}
	// The VPN did not name its interface (e.g. WireGuard asks the kernel for "utun" and the number is chosen later):
	// take the tunnel that carries the default route.
	for _, c := range all {
		if ok(c) {
			return c
		}
	}
	return ""
}

// UtunInterfaces filters the output of `ifconfig -l` down to utunN names, in numeric order.
func UtunInterfaces(ifconfigList string) []string {
	var out []string
	for _, n := range strings.Fields(ifconfigList) {
		if utunExactRe.MatchString(n) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(strings.TrimPrefix(out[i], "utun"))
		b, _ := strconv.Atoi(strings.TrimPrefix(out[j], "utun"))
		return a < b
	})
	return out
}

// OnLink reports whether addr lies inside one of the given directly attached subnets.
func OnLink(addr string, subnets []netip.Prefix) bool {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	for _, p := range subnets {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
