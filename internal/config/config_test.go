// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverlaysDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"notify": false, "loopbackBlockPorts": [1234]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Load(p)
	if c.Notify || len(c.LoopbackBlockPorts) != 1 || c.LoopbackBlockPorts[0] != 1234 {
		t.Errorf("overrides not applied: %+v", c)
	}
	if c.GroupName != "_netleash" || !c.EnforceLauncherOnly || !c.AutoDetectVPNDNS || c.VPNAppPath == "" {
		t.Errorf("defaults lost: %+v", c)
	}
}

func TestLoadMissingOrBrokenFileGivesDefaults(t *testing.T) {
	d := t.TempDir()
	if c := Load(filepath.Join(d, "nope.json")); c.GroupName != "_netleash" {
		t.Errorf("missing file: %+v", c)
	}
	p := filepath.Join(d, "bad.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o644)
	if c := Load(p); c.GroupName != "_netleash" || !c.Notify {
		t.Errorf("broken file must yield defaults: %+v", c)
	}
}

func TestVPNAppsMergesBothForms(t *testing.T) {
	c := Default()
	if len(c.VPNApps()) != 2 {
		t.Errorf("default must list both Amnezia apps once each: %v", c.VPNApps())
	}
	c.VPNAppPaths = nil
	c.VPNAppPath = "/Applications/Other.app"
	if got := c.VPNApps(); len(got) != 1 || got[0] != "/Applications/Other.app" {
		t.Errorf("old single-path config must still work: %v", got)
	}
	c.VPNAppPath = ""
	if len(c.VPNApps()) != 0 {
		t.Errorf("no apps configured: %v", c.VPNApps())
	}
}
