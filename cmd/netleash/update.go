// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sedooj/netleash/internal/semver"
	"github.com/sedooj/netleash/internal/update"
)

const updateUsage = `usage: netleash update [flags]

Checks GitHub for a newer release and, after you confirm, downloads it, verifies its SHA-256 checksum and runs its
installer with sudo. By default all network traffic goes THROUGH THE VPN (it is run inside the same network
confinement as the protected app), so GitHub and your ISP do not see this Mac's real address.

  --check            only report whether an update is available
  --json             machine-readable output (used by the menu bar app)
  --yes, -y          do not ask for confirmation
  --direct           connect outside the VPN (works without VPN, but the connection is not protected)
  --download-only    download and verify, print where the installer is, do not install
  --version vX.Y.Z   take this release instead of the latest one
`

type updateResult struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Update    bool   `json:"update"`
	URL       string `json:"url,omitempty"`
	Dir       string `json:"dir,omitempty"`
	Installer string `json:"installer,omitempty"`
	Error     string `json:"error,omitempty"`
}

func userAgent() string { return "netleash/" + version }

// directFetcher connects without the VPN confinement.
func directFetcher(url string) ([]byte, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream, */*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s for %s", resp.Status, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

// confinedFetcher runs curl through the setuid helper: same group, same pf rules as the protected app, so the request
// can only leave through the VPN tunnel (and fails if the VPN is down).
func confinedFetcher(url string) ([]byte, error) {
	cmd := exec.Command(helperPath, "/usr/bin/curl", "-fsSL", "--max-time", "90", "-A", userAgent(), url)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if strings.Contains(msg, "protection is not active") {
			return nil, errors.New("protection is not active: " + msg)
		}
		return nil, errors.New("could not reach GitHub through the VPN (is it connected?); use --direct to connect outside the VPN")
	}
	return out.Bytes(), nil
}

func cmdUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "")
	asJSON := fs.Bool("json", false, "")
	yes := fs.Bool("yes", false, "")
	fs.BoolVar(yes, "y", false, "")
	direct := fs.Bool("direct", false, "")
	downloadOnly := fs.Bool("download-only", false, "")
	pin := fs.String("version", "", "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, updateUsage)
		return 64
	}

	res := updateResult{Current: strings.TrimPrefix(version, "v")}
	fail := func(err error) int {
		res.Error = err.Error()
		if *asJSON {
			emit(res)
		} else {
			fmt.Fprintln(os.Stderr, "netleash update:", err)
		}
		return 1
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fail(fmt.Errorf("release archives are published only for darwin/arm64 (this is %s/%s)", runtime.GOOS, runtime.GOARCH))
	}
	fetch := update.Fetcher(confinedFetcher)
	if *direct {
		fetch = directFetcher
	}

	tag := *pin
	if tag == "" {
		rel, err := update.Latest(fetch)
		if err != nil {
			return fail(err)
		}
		tag, res.URL = rel.Tag, rel.URL
	}
	res.Latest = strings.TrimPrefix(tag, "v")
	res.Update = *pin != "" || semver.Newer(res.Current, res.Latest)

	if !res.Update {
		if *asJSON {
			emit(res)
		} else {
			fmt.Printf("Netleash %s is up to date.\n", res.Current)
		}
		return 0
	}
	if !*asJSON {
		fmt.Printf("Update available: %s → %s\n", res.Current, res.Latest)
		if res.URL != "" {
			fmt.Println("Release notes:", res.URL)
		}
	}
	if *check {
		if *asJSON {
			emit(res)
		}
		return 0
	}
	if !*yes && !*downloadOnly && !confirm("Download, verify and install it now? [y/N] ") {
		fmt.Println("Aborted.")
		return 0
	}

	asset := update.AssetName(runtime.GOOS, runtime.GOARCH)
	if !*asJSON {
		fmt.Println("Downloading", asset, "...")
	}
	archive, err := update.Download(fetch, tag, asset)
	if err != nil {
		return fail(err)
	}
	dir, err := os.MkdirTemp("", "netleash-update-")
	if err != nil {
		return fail(err)
	}
	if err := update.Extract(archive, dir); err != nil {
		os.RemoveAll(dir)
		return fail(err)
	}
	installer := filepath.Join(dir, strings.TrimSuffix(asset, ".tar.gz"), "install.sh")
	if _, err := os.Stat(installer); err != nil {
		os.RemoveAll(dir)
		return fail(errors.New("unexpected archive layout"))
	}
	res.Dir, res.Installer = dir, installer
	if !*asJSON {
		fmt.Println("Checksum verified.")
	}
	if *downloadOnly {
		if *asJSON {
			emit(res)
		} else {
			fmt.Println("Installer:", installer)
		}
		return 0
	}

	defer os.RemoveAll(dir)
	cmd := exec.Command("/usr/bin/sudo", installer, "-y")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fail(fmt.Errorf("installer failed: %w", err))
	}
	return 0
}

func emit(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}
