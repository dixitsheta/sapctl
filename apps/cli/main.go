// Package main is the entry point for the sapctl CLI.
//
// sapctl is the unified, agent-native command-line interface for the SAP
// product portfolio.
package main

import (
	"fmt"
	"os"

	"github.com/dixitsheta/sapctl/apps/cli/cmd"
	"github.com/dixitsheta/sapctl/apps/cli/internal/config"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := config.LoadDefault(); err != nil {
		fmt.Fprintln(os.Stderr, "sapctl: warning loading ~/.config/sapctl/.env:", err)
	}
	os.Exit(cmd.Execute(version, os.Stdout, os.Stderr))
}
