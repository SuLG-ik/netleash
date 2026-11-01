// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sedooj/netleash/internal/config"
	"github.com/sedooj/netleash/internal/netinfo"
)

var (
	helperPath = config.HelperPath
	logPath    = config.LogPath
)

type status struct {
	Armed bool    `json:"armed"`
	VPNUp bool    `json:"vpnUp"`
	Iface string  `json:"iface"`
	TS    float64 `json:"ts"`
	PID   int     `json:"pid"`
	Note  string  `json:"note"`
}

func cmdStatus() int {
	data, err := os.ReadFile(config.StatusPath)
	if err != nil {
		fmt.Printf("✗ daemon is not running (no %s)\n", config.StatusPath)
		return 2
	}
	var st status
	if json.Unmarshal(data, &st) != nil {
		fmt.Println("✗ unreadable status file")
		return 2
	}
	if age := time.Since(time.Unix(0, int64(st.TS*1e9))); age > 5*time.Second {
		fmt.Printf("✗ daemon has not updated its status for %ds\n", int(age.Seconds()))
		return 2
	}
	if !st.Armed {
		fmt.Printf("✗ protection is NOT armed: %s\n", st.Note)
		return 2
	}
	if st.VPNUp {
		fmt.Printf("● VPN connected (%s) — network allowed\n", st.Iface)
	} else {
		fmt.Println("○ VPN disconnected — network of the protected app is blocked")
	}
	fmt.Printf("  protection armed, daemon pid %d\n", st.PID)
	return 0
}

// cmdTest is the leak test: everything from the confined group except the VPN interface must fail.
func cmdTest() int {
	if code := cmdStatus(); code != 0 {
		return code
	}
	fmt.Println("→ through the VPN (must work if the VPN is up):")
	out, err := exec.Command(helperPath, "/usr/bin/curl", "-s", "-m", "8", "-o", "/dev/null",
		"-w", "   api.anthropic.com HTTP %{http_code}\n", "https://api.anthropic.com").Output()
	if err != nil {
		fmt.Println("   no answer")
	} else {
		fmt.Print(string(out))
	}
	fmt.Println("→ forced through a physical interface (MUST be blocked):")
	leak := false
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if !netinfo.IsPhysicalInterface(i.Name) || !hasIPv4(i) {
			continue
		}
		err := exec.Command(helperPath, "/usr/bin/curl", "-s", "-m", "5", "--interface", i.Name, "-o", "/dev/null", "https://1.1.1.1").Run()
		if err == nil {
			fmt.Printf("   ✗ LEAK through %s!\n", i.Name)
			leak = true
		} else {
			fmt.Printf("   ✓ %s is blocked\n", i.Name)
		}
	}
	if leak {
		fmt.Println("✗ LEAKS FOUND")
		return 1
	}
	fmt.Println("✓ no leaks found")
	return 0
}

func hasIPv4(i net.Interface) bool {
	addrs, _ := i.Addrs()
	for _, a := range addrs {
		if ip, _, err := net.ParseCIDR(a.String()); err == nil && ip.To4() != nil {
			return true
		}
	}
	return false
}

func cmdResume() int {
	cmd := exec.Command("/usr/bin/sudo", filepath.Join(config.LibexecDir, "resume.sh"))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	return cmdStatus()
}

func execSudo(script string) {
	execTool("/usr/bin/sudo", filepath.Join(config.LibexecDir, script))
}

// execTool replaces the current process (no PATH lookup: absolute paths only).
func execTool(path string, args ...string) {
	if err := syscall.Exec(path, append([]string{path}, args...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "netleash: cannot run %s: %v\n", path, err)
		os.Exit(127)
	}
}
