// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Command netleash-exec is a setuid-root helper. It runs a command as the CALLING user, but with a single group
// (the confined group; all supplementary groups are dropped and the original gid cannot be regained).
// The pf anchor maintained by the daemon restricts the network of that group. The helper only ever LOWERS
// privileges, so running an arbitrary command through it grants nothing beyond what the user already has.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Set at build time: -ldflags "-X main.version=0.1.0".
var version = "dev"

const (
	statusPath = "/var/run/netleash/status.json"
	spawnTool  = "/usr/local/libexec/netleash/netleash-spawn"
)

func die(code int, format string, a ...any) {
	fmt.Fprintf(os.Stderr, "netleash-exec: "+format+"\n", a...)
	os.Exit(code)
}

// trustedRead reads a file only if it is a regular, root-owned file that nobody else can write, in a directory
// with the same properties: the status file is the source of the confined gid and must not be forgeable.
func trustedRead(path string) ([]byte, error) {
	for _, p := range []string{filepath.Dir(path), path} {
		fi, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || st.Uid != 0 || fi.Mode().Perm()&0o022 != 0 || (p == path && !fi.Mode().IsRegular()) || (p != path && !fi.IsDir()) {
			return nil, fmt.Errorf("%s is not trusted", p)
		}
	}
	return os.ReadFile(path)
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("netleash-exec", version)
		return
	}
	args := os.Args[1:]
	// --disclaim: run the program through netleash-spawn, so macOS treats it as its own "responsible process" (privacy
	// prompts then name the program itself instead of the app that launched it).
	disclaim := len(args) > 0 && args[0] == "--disclaim"
	if disclaim {
		args = args[1:]
	}
	if len(args) < 1 {
		die(64, "usage: netleash-exec [--disclaim] <program> [arguments...]")
	}
	if os.Geteuid() != 0 {
		die(1, "not setuid root (run the installer)")
	}
	realUID, origGID := os.Getuid(), os.Getgid()
	if realUID == 0 {
		die(1, "do not run as root")
	}

	// The daemon must be alive and armed, otherwise we refuse to start. The confined gid comes from the same file.
	data, err := trustedRead(statusPath)
	var st struct {
		Armed bool    `json:"armed"`
		TS    float64 `json:"ts"`
		GID   uint32  `json:"gid"`
	}
	if err != nil || json.Unmarshal(data, &st) != nil || !st.Armed || st.GID == 0 ||
		time.Since(time.Unix(0, int64(st.TS*1e9))) > 5*time.Second {
		die(2, "protection is not active (daemon not running or pf not armed). Check: netleash status")
	}
	gid := int(st.GID)

	// Order matters: groups first, then gid, then uid (root is lost after setuid).
	if err := syscall.Setgroups([]int{gid}); err != nil {
		die(1, "setgroups: %v", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		die(1, "setgid: %v", err)
	}
	if err := syscall.Setuid(realUID); err != nil {
		die(1, "setuid: %v", err)
	}

	// Verify that the privilege drop is irreversible.
	if syscall.Setuid(0) == nil || syscall.Setgid(origGID) == nil || syscall.Setgid(20) == nil {
		die(3, "could not drop privileges irreversibly")
	}
	if os.Getgid() != gid || os.Getegid() != gid || os.Getuid() != realUID || os.Geteuid() != realUID {
		die(3, "unexpected uid/gid after drop")
	}

	if disclaim {
		// The spawn tool lives in a root-owned directory; it runs as the user with the group set above.
		args = append([]string{spawnTool}, args...)
	}
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		die(127, "exec %s: %v", args[0], err)
	}
}
