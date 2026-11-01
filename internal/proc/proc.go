// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package proc lists processes with their effective gid and executable path.
package proc

// Proc is one running process.
type Proc struct {
	PID  int
	GID  uint32 // effective gid (what `ps -o gid` shows, and what pf matches)
	Path string // executable path
}
