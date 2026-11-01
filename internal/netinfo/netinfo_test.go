// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package netinfo

import (
	"net/netip"
	"reflect"
	"sort"
	"testing"
)

// Trimmed real-world outputs (addresses are documentation-style or the VPN's private ones).
const netstat = `Routing tables

Internet:
Destination        Gateway            Flags               Netif Expire
default            172.20.10.1        UGScg                 en0
0/1                172.20.10.1        UGScIg                en0
1                  10.33.0.2          UGSc               utun22
128.0/1            10.33.0.2          UGSc               utun22
128.0/1            172.20.10.1        UGScI                 en0
8.8.4.4            10.33.0.2          UGHS               utun22
100.64.0.1         10.33.0.2          UGHS               utun22
127.0.0.1          127.0.0.1          UH                   lo0
`

const scutil = `DNS configuration

resolver #1
  nameserver[0] : 100.64.0.1
  nameserver[1] : 8.8.4.4
  flags    : Request A records
resolver #2
  domain   : local
`

const ifconfigEn0 = `en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	ether 00:00:5e:00:53:01
	inet 172.20.10.2 netmask 0xfffffff0 broadcast 172.20.10.15
`

func TestHasDefaultRoute(t *testing.T) {
	if !HasDefaultRoute(netstat, "utun22") {
		t.Error("utun22 carries 128.0/1")
	}
	if HasDefaultRoute(netstat, "en0") != true {
		t.Error("en0 has an unscoped default route")
	}
	if HasDefaultRoute(netstat, "utun9") {
		t.Error("unknown interface must not have a default route")
	}
	// ifscope-only routes must not count
	if HasDefaultRoute("0/1  1.2.3.4  UGScIg  en5\n", "en5") {
		t.Error("interface-scoped route must be ignored")
	}
}

func TestVPNDNSAutoDetection(t *testing.T) {
	servers := Nameservers(scutil)
	via := HostRoutesVia(netstat, "utun22")
	got := FilterVPNDNS(servers, via, Subnets(ifconfigEn0))
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"100.64.0.1"}) {
		t.Fatalf("want only the CGNAT DNS (public 8.8.4.4 is excluded), got %v", got)
	}
}

func TestRouterDNSOnPhysicalSubnetIsNeverBlocked(t *testing.T) {
	servers := map[string]bool{"172.20.10.1": true}
	via := map[string]bool{"172.20.10.1": true}
	if got := FilterVPNDNS(servers, via, Subnets(ifconfigEn0)); len(got) != 0 {
		t.Fatalf("router DNS must stay reachable, got %v", got)
	}
}

func TestIsPrivate(t *testing.T) {
	for s, want := range map[string]bool{
		"10.1.2.3": true, "172.16.0.1": true, "172.32.0.1": false, "192.168.1.1": true, "100.64.0.1": true,
		"100.128.0.1": false, "169.254.1.1": true, "8.8.8.8": false, "1.1.1.1": false,
	} {
		if got := IsPrivate(netip.MustParseAddr(s)); got != want {
			t.Errorf("IsPrivate(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestSubnets(t *testing.T) {
	got := Subnets(ifconfigEn0)
	if len(got) != 1 || got[0] != netip.MustParsePrefix("172.20.10.0/28") {
		t.Fatalf("unexpected subnets %v", got)
	}
}

func TestUtunNamesAndPhysical(t *testing.T) {
	if got := UtunNames("tun2socks -device tun://utun22 -proxy x utun22 utun3"); !reflect.DeepEqual(got, []string{"utun22", "utun3"}) {
		t.Errorf("UtunNames = %v", got)
	}
	if got := PhysicalInterfaces("lo0 gif0 en4 en0 awdl0 bridge0 utun1 en10"); !reflect.DeepEqual(got, []string{"en4", "en0", "en10"}) {
		t.Errorf("PhysicalInterfaces = %v", got)
	}
}

const ifUp = "utun7: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1420\n\tinet 10.2.0.2 --> 10.2.0.2 netmask 0xffffffff\n"
const ifDown = "utun3: flags=8010<POINTOPOINT,MULTICAST> mtu 1380\n"

func TestPickTunnelFallsBackToTheTunnelWithTheDefaultRoute(t *testing.T) {
	// WireGuard: the VPN app's command line only says "utun", so there is no explicit name.
	routes := "0/1  10.2.0.1  UGSc  utun7\n128.0/1  10.2.0.1  UGSc  utun7\ndefault 172.20.10.1 UGScg en0\n"
	of := func(n string) string {
		if n == "utun7" {
			return ifUp
		}
		return ifDown
	}
	if got := PickTunnel(nil, []string{"utun3", "utun7"}, routes, of); got != "utun7" {
		t.Errorf("got %q, want utun7", got)
	}
	// an explicit name that does not qualify must not block the fallback
	if got := PickTunnel([]string{"utun3"}, []string{"utun3", "utun7"}, routes, of); got != "utun7" {
		t.Errorf("got %q, want utun7", got)
	}
	// explicit and qualifying wins
	if got := PickTunnel([]string{"utun7"}, []string{"utun3", "utun7"}, routes, of); got != "utun7" {
		t.Errorf("got %q, want utun7", got)
	}
}

func TestPickTunnelNeedsUpAddressAndDefaultRoute(t *testing.T) {
	routes := "10.2.0.1  10.2.0.1  UH  utun7\n" // a host route is not a default route
	if got := PickTunnel(nil, []string{"utun7"}, routes, func(string) string { return ifUp }); got != "" {
		t.Errorf("no default route: got %q, want none", got)
	}
	full := "0/1 10.2.0.1 UGSc utun7\n"
	if got := PickTunnel(nil, []string{"utun7"}, full, func(string) string { return ifDown }); got != "" {
		t.Errorf("interface down: got %q, want none", got)
	}
	if got := PickTunnel(nil, nil, full, func(string) string { return ifUp }); got != "" {
		t.Errorf("no tunnels at all: got %q, want none", got)
	}
}

func TestUtunInterfacesAreSortedNumerically(t *testing.T) {
	got := UtunInterfaces("lo0 utun10 en0 utun2 utun1 utunX awdl0 utun22")
	want := []string{"utun1", "utun2", "utun10", "utun22"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOnLink(t *testing.T) {
	subnets := []netip.Prefix{netip.MustParsePrefix("100.64.0.0/10")} // e.g. a CGNAT network where the gateway is 100.64.0.1
	if !OnLink("100.64.0.1", subnets) {
		t.Error("an address inside an attached subnet is on-link")
	}
	if OnLink("100.64.0.1", []netip.Prefix{netip.MustParsePrefix("172.20.10.0/28")}) || OnLink("garbage", subnets) {
		t.Error("off-link or invalid addresses are not on-link")
	}
}
