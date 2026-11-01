// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package main

import (
	"errors"
	"net/netip"
)

var errNotIPv4 = errors.New("not an IPv4 address")

type netipPrefix = netip.Prefix

func parseAddr(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err == nil && !a.Is4() {
		return a, errNotIPv4
	}
	return a, err
}
