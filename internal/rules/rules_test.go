// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package rules

import (
	"strings"
	"testing"
)

func lines(s string) []string { return strings.Split(strings.TrimSpace(s), "\n") }

func TestVPNUpExactRules(t *testing.T) {
	got := Build(Params{
		Group: "_netleash", VPNIface: "utun22", PhysicalIfaces: []string{"en0", "en1"},
		VPNDNS: []string{"100.64.0.1"}, LoopbackBlockPorts: []int{7890, 9090},
	})
	want := `block drop out quick on { en0, en1 } proto { tcp, udp } from any to { 100.64.0.1 } port 53
block drop quick on lo0 proto { tcp, udp } from any to any port { 7890, 9090 } group _netleash
pass quick on lo0 proto { tcp, udp } all group _netleash no state
pass quick on utun22 proto { tcp, udp } all group _netleash keep state (if-bound)
block drop quick proto { tcp, udp } all group _netleash
`
	if got != want {
		t.Fatalf("unexpected ruleset:\n%s\nwant:\n%s", got, want)
	}
}

// The fail-closed core: whatever the input, the group's traffic ends in a block rule, and the only pass rules
// for the group are loopback and the live tunnel.
func TestFailClosedInvariants(t *testing.T) {
	cases := []Params{
		{Group: "_g"},
		{Group: "_g", VPNIface: "utun3"},
		{Group: "_g", PhysicalIfaces: []string{"en0"}, DNSLock: true},
		{Group: "_g", VPNIface: "utun9", PhysicalIfaces: []string{"en0"}, VPNDNS: []string{"10.0.0.1"}, LoopbackBlockPorts: []int{1}},
	}
	for _, p := range cases {
		ls := lines(Build(p))
		if last := ls[len(ls)-1]; last != "block drop quick proto { tcp, udp } all group "+p.Group {
			t.Errorf("last rule must block the group, got %q", last)
		}
		for _, l := range ls {
			if !strings.HasPrefix(l, "pass") {
				continue
			}
			if !strings.Contains(l, "on lo0 ") && !(p.VPNIface != "" && strings.Contains(l, "on "+p.VPNIface+" ")) {
				t.Errorf("unexpected pass rule: %q", l)
			}
			if !strings.Contains(l, "group "+p.Group) {
				t.Errorf("pass rule without group match: %q", l)
			}
		}
	}
}

func TestVPNDownHasNoTunnelPass(t *testing.T) {
	for _, l := range lines(Build(Params{Group: "_g", PhysicalIfaces: []string{"en0"}})) {
		if strings.HasPrefix(l, "pass") && !strings.Contains(l, "on lo0 ") {
			t.Errorf("no pass rule except loopback expected while the VPN is down: %q", l)
		}
	}
}

func TestDNSLock(t *testing.T) {
	p := Params{Group: "_g", PhysicalIfaces: []string{"en0"}}
	if strings.Contains(Build(p), "{ 53, 853 }") {
		t.Error("DNS lock must be off by default")
	}
	p.DNSLock = true
	if !strings.Contains(Build(p), "block drop out quick on { en0 } proto { tcp, udp } from any to any port { 53, 853 }") {
		t.Error("DNS lock rule missing")
	}
}

func TestNoPhysicalInterfacesMeansNoDNSRules(t *testing.T) {
	out := Build(Params{Group: "_g", VPNDNS: []string{"100.64.0.1"}, DNSLock: true})
	if strings.Contains(out, "port 53") {
		t.Errorf("DNS rules need interfaces to attach to:\n%s", out)
	}
}
