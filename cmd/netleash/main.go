// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Command netleash is the command-line interface and the root daemon of Netleash.
package main

import (
	"fmt"
	"os"
	"runtime"
)

// Set at build time by build.sh: -ldflags "-X main.version=0.1.0 -X main.commit=abc1234 -X main.date=2026-01-01T00:00:00Z".
// Release builds carry a clean semantic version; local builds are marked 0.1.0-dev+<commit>.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func versionString() string {
	s := "netleash " + version
	if commit != "" || date != "" {
		s += fmt.Sprintf(" (commit %s, built %s, %s/%s)", commit, date, runtime.GOOS, runtime.GOARCH)
	}
	return s
}

const usage = `netleash — confine an application's network access to a VPN tunnel

usage: netleash <command>

  status              is protection armed, is the VPN alive
  test                leak test: a forced connection through a physical interface MUST be blocked
  panic               emergency rollback: remove all protection (asks for sudo)
  resume              restore protection after panic (asks for sudo)
  update [flags]      check for a newer release and install it (see: netleash update --help)
  log                 follow the daemon log
  run <command...>    run a command under the same network confinement
  version             print the version
  daemon              run the root daemon (started by launchd; not for manual use)
`

func main() {
	cmd := "status"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "daemon":
		runDaemon()
	case "status":
		os.Exit(cmdStatus())
	case "test":
		os.Exit(cmdTest())
	case "panic":
		execSudo("panic.sh")
	case "resume":
		os.Exit(cmdResume())
	case "update":
		if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Print(updateUsage)
			return
		}
		os.Exit(cmdUpdate(os.Args[2:]))
	case "log":
		execTool("/usr/bin/tail", "-f", logPath)
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(64)
		}
		execTool(helperPath, os.Args[2:]...)
	case "version", "--version", "-v":
		fmt.Println(versionString())
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(64)
	}
}
