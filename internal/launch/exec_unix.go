// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build !windows

package launch

import "syscall"

// execProcess replaces the current process with claude so the terminal, signals
// and exit code pass through transparently (ideal for an interactive TUI).
func execProcess(bin string, argv, env []string) error {
	return syscall.Exec(bin, argv, env)
}
