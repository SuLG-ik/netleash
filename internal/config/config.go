// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package config holds the daemon configuration and the well-known install paths.
package config

import (
	"encoding/json"
	"os"
)

// Install layout. Everything is derived from Name so a fork can rename the project in one place.
const (
	Name       = "netleash"
	LibexecDir = "/usr/local/libexec/" + Name
	ConfigDir  = "/usr/local/etc/" + Name
	StateDir   = "/var/run/" + Name
	LogPath    = "/var/log/" + Name + ".log"

	ConfigPath     = ConfigDir + "/config.json"
	LearnedDNSPath = ConfigDir + "/learned-dns.json"
	PFTokenPath    = ConfigDir + "/pf-token"
	StatusPath     = StateDir + "/status.json"
	HelperPath     = LibexecDir + "/" + Name + "-exec"
	DaemonLabel    = "org.netleash.daemon"

	// AnchorName lives inside Apple's com.apple/* namespace: the stock /etc/pf.conf already references it, so other
	// programs reloading the main ruleset (e.g. a VPN kill switch) do not remove our rules.
	AnchorName = "com.apple/050." + Name
)

// Config is read from ConfigPath. Every key is optional; missing keys keep their defaults.
type Config struct {
	// GroupName is the macOS group whose network is confined.
	GroupName string `json:"groupName"`
	// ProtectedAppPath is the application whose network access is restricted.
	ProtectedAppPath string `json:"protectedAppPath"`
	// VPNAppPath is the VPN application whose tunnel interface is the only permitted route.
	VPNAppPath string `json:"vpnAppPath"`
	// EnforceLauncherOnly kills protected-app processes that were not started through the launcher.
	EnforceLauncherOnly bool `json:"enforceLauncherOnly"`
	Notify              bool `json:"notify"`
	// LoopbackBlockPorts are local proxy ports the protected app must not use (they could bypass the VPN).
	LoopbackBlockPorts []int `json:"loopbackBlockPorts"`
	// IgnoreCommandSubstrings are paths inside the app that are not killed as "unprotected launch". The Squirrel updater
	// is started by launchd with another gid, does not use the network, and launchd restarts it if it is killed.
	IgnoreCommandSubstrings []string `json:"ignoreCommandSubstrings"`
	// VPNDNSServers are DNS servers of the VPN that must never be reachable through physical interfaces (manual part).
	VPNDNSServers []string `json:"vpnDnsServers"`
	// AutoDetectVPNDNS extends VPNDNSServers with private addresses from the system resolvers that have a host route
	// via the VPN interface.
	AutoDetectVPNDNS bool `json:"autoDetectVpnDns"`
}

// Default returns the configuration used when no config file exists.
func Default() Config {
	return Config{
		GroupName:               "_" + Name,
		ProtectedAppPath:        "/Applications/Claude.app",
		VPNAppPath:              "/Applications/AmneziaVPN.app",
		EnforceLauncherOnly:     true,
		Notify:                  true,
		LoopbackBlockPorts:      []int{7890, 7891, 7892, 7897, 9090, 9097},
		IgnoreCommandSubstrings: []string{"/Squirrel.framework/"},
		VPNDNSServers:           []string{"100.64.0.1"},
		AutoDetectVPNDNS:        true,
	}
}

// Load reads path over the defaults. A missing or invalid file yields the defaults.
func Load(path string) Config {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if json.Unmarshal(data, &c) != nil {
		return Default()
	}
	return c
}
