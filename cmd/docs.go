// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manDate pins the man-page .TH date so generation is byte-for-byte
// reproducible. Cobra otherwise stamps time.Now(), churning every page's date
// header on each regeneration. Bump this on a meaningful documentation revision.
var manDate = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

// docsCmd builds the hidden docs command that regenerates the committed CLI
// reference (docs/cli) from the command tree. Hidden because it is a
// maintainer task driven by `make docs`, not part of the user-facing surface.
// The extension is distributed only through `gh extension install`, so man
// pages are not shipped, but the format stays available for ad-hoc use.
func docsCmd() *cobra.Command {
	var out, format string
	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate the CLI reference from the command tree.",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			root.DisableAutoGenTag = true // keep the output reproducible
			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", out, err)
			}
			switch format {
			case "markdown":
				return doc.GenMarkdownTree(root, out)
			case "man":
				return doc.GenManTree(root, &doc.GenManHeader{Title: "GH-CLAUDE", Section: "1", Date: &manDate}, out)
			case "rest":
				return doc.GenReSTTree(root, out)
			default:
				return fmt.Errorf("unknown format %q (expected markdown, man, or rest)", format)
			}
		},
	}
	cmd.Flags().StringVar(&out, "out", "docs/cli", "directory to write the reference into")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown, man, or rest")
	return cmd
}
