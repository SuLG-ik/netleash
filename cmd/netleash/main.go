// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors

// Command netleash is the command-line interface and the root daemon of Netleash.
package main

import (
	"fmt"
	"os"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

const usage = `netleash — confine an application's network access to a VPN tunnel

usage: netleash <command>

  status              is protection armed, is the VPN alive
  test                leak test: a forced connection through a physical interface MUST be blocked
  panic               emergency rollback: remove all protection (asks for sudo)
  resume              restore protection after panic (asks for sudo)
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
	case "log":
		execTool("/usr/bin/tail", "-f", logPath)
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(64)
		}
		execTool(helperPath, os.Args[2:]...)
	case "version", "--version", "-v":
		fmt.Println("netleash", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(64)
	}
}
