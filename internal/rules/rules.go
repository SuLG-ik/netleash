// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package rules generates the pf anchor ruleset. It is a pure function of its input, which makes the security
// properties of the generated rules testable.
package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// Params describes the desired state of the anchor.
type Params struct {
	Group              string   // confined group, e.g. "_netleash"
	VPNIface           string   // live tunnel interface, "" when the VPN is down
	PhysicalIfaces     []string // en0, en1, … (DNS must not leave through them)
	VPNDNS             []string // VPN DNS servers that may never be queried through a physical interface
	DNSLock            bool     // block all DNS on physical interfaces (VPN down and protected app running)
	LoopbackBlockPorts []int    // local proxy ports the confined group must not reach
}

func list(items []string) string { return "{ " + strings.Join(items, ", ") + " }" }

// Build returns the anchor ruleset.
//
// Invariants (checked by tests): the last rule always blocks all tcp/udp of the group, and the only
// pass rules for the group are loopback and the live VPN interface.
func Build(p Params) string {
	g := "group " + p.Group
	var b strings.Builder
	// System DNS is resolved by mDNSResponder, which is not in our group, so these two rules apply to everyone:
	// (a) the VPN's own DNS servers may never be queried through a physical interface (this happens briefly while
	//     the VPN connects: the resolver switches before the tunnel route exists);
	// (b) while the VPN is down and the protected app is running, no hostname may leave on a physical interface.
	if len(p.PhysicalIfaces) > 0 {
		en := list(p.PhysicalIfaces)
		if len(p.VPNDNS) > 0 {
			fmt.Fprintf(&b, "block drop out quick on %s proto { tcp, udp } from any to %s port 53\n", en, list(p.VPNDNS))
		}
		if p.DNSLock {
			fmt.Fprintf(&b, "block drop out quick on %s proto { tcp, udp } from any to any port { 53, 853 }\n", en)
		}
	}
	if len(p.LoopbackBlockPorts) > 0 {
		ports := make([]string, len(p.LoopbackBlockPorts))
		for i, port := range p.LoopbackBlockPorts {
			ports[i] = strconv.Itoa(port)
		}
		fmt.Fprintf(&b, "block drop quick on lo0 proto { tcp, udp } from any to any port %s %s\n", list(ports), g)
	}
	fmt.Fprintf(&b, "pass quick on lo0 proto { tcp, udp } all %s no state\n", g)
	if p.VPNIface != "" {
		fmt.Fprintf(&b, "pass quick on %s proto { tcp, udp } all %s keep state (if-bound)\n", p.VPNIface, g)
	}
	fmt.Fprintf(&b, "block drop quick proto { tcp, udp } all %s\n", g)
	return b.String()
}
