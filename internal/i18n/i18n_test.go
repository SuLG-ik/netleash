// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package i18n

import (
	"regexp"
	"strings"
	"testing"
)

var verb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

// A translation with different format verbs than its English original would corrupt the output (or panic at runtime).
func TestTranslationsKeepFormatVerbs(t *testing.T) {
	for en, ruText := range ru {
		if strings.TrimSpace(ruText) == "" {
			t.Errorf("empty translation for %q", en)
		}
		a, b := verb.FindAllString(en, -1), verb.FindAllString(ruText, -1)
		if len(a) != len(b) {
			t.Errorf("verbs differ: %q (%v) vs %q (%v)", en, a, ruText, b)
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("verbs differ: %q vs %q", en, ruText)
			}
		}
	}
}

func TestTranslateAndFallback(t *testing.T) {
	defer Set("en")
	Set("ru")
	if got := T("✗ daemon has not updated its status for %ds", 7); got != "✗ демон не обновлял статус 7 с" {
		t.Errorf("got %q", got)
	}
	if got := T("a message nobody translated %d", 3); got != "a message nobody translated 3" {
		t.Errorf("untranslated text must fall back to English, got %q", got)
	}
	Set("en")
	if got := T("Aborted."); got != "Aborted." {
		t.Errorf("got %q", got)
	}
}

func TestNorm(t *testing.T) {
	for in, want := range map[string]string{"ru_RU.UTF-8": "ru", "RU": "ru", "en_US.UTF-8": "en", "de_DE": "en", "": "en"} {
		if got := norm(in); got != want {
			t.Errorf("norm(%q) = %q, want %q", in, got, want)
		}
	}
}
