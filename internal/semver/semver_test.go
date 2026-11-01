// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package semver

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.1.0", "v0.1.1", true},
		{"0.1.0", "0.1.0", false},
		{"0.2.0", "0.1.9", false},
		{"0.1.0-dev+abc.dirty", "0.1.0", true}, // a local dev build is older than the release
		{"0.1.0", "0.1.0-rc1", false},          // a pre-release is older than its release
		{"0.1.0-rc1", "0.1.0-rc2", true},
		{"0.1.0-rc.2", "0.1.0-rc.10", true}, // dot-separated numeric identifiers compare as numbers
		{"0.1.0-rc2", "0.1.0-rc10", false},  // "rc2" is one alphanumeric identifier: lexical order, as SemVer defines
		{"1.0.0-alpha", "1.0.0-alpha.1", true},
		{"1.0.0-alpha.1", "1.0.0-beta", true},
		{"dev", "0.0.1", true}, // unparsable = oldest
		{"1.9.0", "1.10.0", true},
		{"0.1.0+build5", "0.1.0+build9", false}, // build metadata is ignored
	}
	for _, c := range cases {
		if got := Newer(c.current, c.candidate); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.candidate, got, c.want)
		}
	}
}

func TestParseString(t *testing.T) {
	if got := Parse("v1.2.3-rc.1+x").String(); got != "1.2.3-rc.1" {
		t.Errorf("got %q", got)
	}
	if got := Parse("garbage").String(); got != "0.0.0-dev" {
		t.Errorf("got %q", got)
	}
}
