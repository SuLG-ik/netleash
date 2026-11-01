// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package sys runs external system tools with a timeout.
package sys

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// Timeout bounds every external command so a hung tool cannot stall the daemon.
const Timeout = 10 * time.Second

// Run executes name with args and returns its combined output and success flag.
func Run(name string, args ...string) (string, bool) {
	return RunStdin("", name, args...)
}

// RunStdin is Run with data written to the command's standard input.
func RunStdin(stdin string, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err == nil
}
