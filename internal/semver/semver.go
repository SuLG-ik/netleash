// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package semver parses and compares Semantic Versions (https://semver.org), enough for update checks.
package semver

import (
	"strconv"
	"strings"
)

// Version is a parsed semantic version. Build metadata (after "+") is ignored in comparisons.
type Version struct {
	Major, Minor, Patch int
	Pre                 []string // pre-release identifiers, empty for a release
}

// Parse accepts "1.2.3", "v1.2.3", "1.2.3-rc1", "1.2.3-dev+abc.dirty". Anything unparsable (e.g. "dev") is 0.0.0-dev.
func Parse(s string) Version {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s, pre = s[:i], s[i+1:]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{Pre: []string{"dev"}}
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{Pre: []string{"dev"}}
		}
		nums[i] = n
	}
	v := Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}
	if pre != "" {
		v.Pre = strings.Split(pre, ".")
	}
	return v
}

// String formats the version without build metadata.
func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	return s
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Compare returns -1, 0 or 1 following the SemVer precedence rules (a pre-release is lower than its release).
func Compare(a, b Version) int {
	if c := cmpInt(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpInt(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpInt(a.Patch, b.Patch); c != 0 {
		return c
	}
	switch {
	case len(a.Pre) == 0 && len(b.Pre) == 0:
		return 0
	case len(a.Pre) == 0:
		return 1
	case len(b.Pre) == 0:
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		x, y := a.Pre[i], b.Pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if c := cmpInt(xn, yn); c != 0 {
				return c
			}
		case xerr == nil: // numeric identifiers are lower than alphanumeric ones
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(a.Pre), len(b.Pre))
}

// Newer reports whether candidate has higher precedence than current.
func Newer(current, candidate string) bool { return Compare(Parse(current), Parse(candidate)) < 0 }
