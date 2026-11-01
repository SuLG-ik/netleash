// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

//go:build !darwin

package proc

import "errors"

// List is only implemented on macOS for now.
func List() ([]Proc, error) { return nil, errors.New("unsupported platform") }

// Args is only implemented on macOS for now.
func Args(pid int) (string, []string) { return "", nil }
