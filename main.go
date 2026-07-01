// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Command gh-claude is a GitHub CLI extension that launches Claude Code with a
// temporary, least-privilege GitHub token (read-only on source code, read/write
// on issues and pull requests) so Claude can work with private repositories
// without ever seeing the user's real credential or the OS keychain.
package main

import (
	"fmt"
	"os"

	"github.com/bitwise-media-group/gh-claude/cmd"
)

// version is the release version, injected at build time via
// -ldflags "-X main.version=…" (see the Makefile and .goreleaser.yaml). It stays
// "dev" for local `go build`, which the integrity check treats as un-gated. The
// value is handed to the cmd package, which owns the command tree.
var version = "dev"

func main() {
	if err := cmd.Root(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
