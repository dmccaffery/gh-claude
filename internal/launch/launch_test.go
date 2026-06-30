// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package launch

import (
	"strings"
	"testing"
)

func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func TestEnvWiresTokenForGhAndGit(t *testing.T) {
	env := envToMap(Env([]string{"PATH=/usr/bin", "HOME=/home/x"}, "tok123"))

	checks := map[string]string{
		"GH_TOKEN":           "tok123",
		"GITHUB_TOKEN":       "tok123",
		"GIT_CONFIG_COUNT":   "2",
		"GIT_CONFIG_KEY_0":   gitCredentialHelperKey,
		"GIT_CONFIG_VALUE_0": "",
		"GIT_CONFIG_KEY_1":   gitCredentialHelperKey,
		"GIT_CONFIG_VALUE_1": "!gh auth git-credential",
		"PATH":               "/usr/bin", // parent env preserved
		"HOME":               "/home/x",
	}
	for k, want := range checks {
		if got := env[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestEnvAppendsToExistingGitConfig(t *testing.T) {
	parent := []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=user.name",
		"GIT_CONFIG_VALUE_0=Existing User",
	}
	env := envToMap(Env(parent, "tok"))

	if got := env["GIT_CONFIG_COUNT"]; got != "3" {
		t.Errorf("GIT_CONFIG_COUNT = %q, want 3", got)
	}
	// Pre-existing entry must be preserved.
	if got := env["GIT_CONFIG_KEY_0"]; got != "user.name" {
		t.Errorf("GIT_CONFIG_KEY_0 = %q, want user.name (preserved)", got)
	}
	// Our entries are appended at indices 1 and 2.
	if got := env["GIT_CONFIG_KEY_1"]; got != gitCredentialHelperKey {
		t.Errorf("GIT_CONFIG_KEY_1 = %q, want %q", got, gitCredentialHelperKey)
	}
	if got := env["GIT_CONFIG_VALUE_1"]; got != "" {
		t.Errorf("GIT_CONFIG_VALUE_1 = %q, want empty (reset)", got)
	}
	if got := env["GIT_CONFIG_KEY_2"]; got != gitCredentialHelperKey {
		t.Errorf("GIT_CONFIG_KEY_2 = %q, want %q", got, gitCredentialHelperKey)
	}
	if got := env["GIT_CONFIG_VALUE_2"]; got != "!gh auth git-credential" {
		t.Errorf("GIT_CONFIG_VALUE_2 = %q, want the gh helper", got)
	}
}
