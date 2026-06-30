// Package launch builds the child environment that wires gh and git to the
// scoped token, then replaces the current process with Claude Code.
package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// gitCredentialHelperKey configures git's credential helper for github.com.
const gitCredentialHelperKey = "credential.https://github.com.helper"

// Env returns the child environment: the parent environment plus the token
// wiring. It sets GH_TOKEN/GITHUB_TOKEN so gh uses the scoped token (and never
// touches the keychain), and injects a git credential helper via git's
// GIT_CONFIG_* environment mechanism so git defers to gh — all without
// mutating any file on disk. Any pre-existing GIT_CONFIG_* entries are
// preserved by appending after them.
func Env(parent []string, token string) []string {
	env := envMap(parent)

	base := 0
	if c, ok := env["GIT_CONFIG_COUNT"]; ok {
		if n, err := strconv.Atoi(c); err == nil && n >= 0 {
			base = n
		}
	}

	env["GH_TOKEN"] = token
	env["GITHUB_TOKEN"] = token
	// First entry resets any inherited helper list; the second installs ours.
	env[fmt.Sprintf("GIT_CONFIG_KEY_%d", base)] = gitCredentialHelperKey
	env[fmt.Sprintf("GIT_CONFIG_VALUE_%d", base)] = ""
	env[fmt.Sprintf("GIT_CONFIG_KEY_%d", base+1)] = gitCredentialHelperKey
	env[fmt.Sprintf("GIT_CONFIG_VALUE_%d", base+1)] = "!gh auth git-credential"
	env["GIT_CONFIG_COUNT"] = strconv.Itoa(base + 2)

	return envSlice(env)
}

// Run replaces the current process with `claude`, launched in the current
// working directory with extraArgs appended and the token wired into the
// environment. On success it does not return.
func Run(token string, extraArgs []string) error {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("could not find the `claude` executable on PATH; install Claude Code first: %w", err)
	}
	argv := append([]string{bin}, extraArgs...)
	return execProcess(bin, argv, Env(os.Environ(), token))
}

func envMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func envSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
