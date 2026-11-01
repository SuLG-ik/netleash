// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Package i18n translates the command-line messages. The English text is the key; a language is chosen from
// NETLEASH_LANG or the usual locale variables (LC_ALL, LC_MESSAGES, LANG). Unknown languages fall back to English.
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var lang = detect()

func norm(v string) string {
	v = strings.ToLower(v)
	if strings.HasPrefix(v, "ru") {
		return "ru"
	}
	return "en"
}

// fileLang reads the language chosen in the menu bar app's settings ("en", "ru"; absent = system default).
func fileLang(home string) string {
	if home == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, "Library", "Application Support", "Netleash", "language"))
	if err != nil {
		return ""
	}
	if v := strings.TrimSpace(string(data)); v == "en" || v == "ru" {
		return v
	}
	return ""
}

func detect() string {
	if v := os.Getenv("NETLEASH_LANG"); v != "" {
		return norm(v)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if v := fileLang(home); v != "" {
			return v
		}
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			return norm(v)
		}
	}
	return "en"
}

// Set forces a language ("en" or "ru"); used by tests.
func Set(l string) { lang = norm(l) }

// T returns the translation of the English format string, formatted with args.
func T(format string, args ...any) string {
	if lang == "ru" {
		if t, ok := ru[format]; ok {
			format = t
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
