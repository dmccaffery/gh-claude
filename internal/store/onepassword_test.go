// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeOP is an in-memory stand-in for the `op` CLI. It records every invocation
// (args and stdin) so tests can assert how the backend shells out, and stores
// items by title so round trips work without a real binary.
type fakeOP struct {
	items      map[string]string // title -> credential value
	calls      []fakeCall
	failWhoami bool
}

type fakeCall struct {
	args  []string
	stdin []byte
}

func newFakeOP() *fakeOP { return &fakeOP{items: map[string]string{}} }

func (f *fakeOP) run(args []string, stdin []byte) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{args: args, stdin: stdin})

	// Strip a leading global --account flag, as the real CLI accepts.
	if len(args) >= 2 && args[0] == "--account" {
		args = args[2:]
	}

	notFound := func(what string) error {
		return &opCmdError{err: errors.New("exit status 1"), stderr: fmt.Sprintf("%q isn't an item", what)}
	}

	switch {
	case len(args) == 1 && args[0] == "whoami":
		if f.failWhoami {
			return nil, &opCmdError{err: errors.New("exit status 1"), stderr: "account is not signed in"}
		}
		return []byte("user@example.com"), nil

	case len(args) >= 1 && args[0] == "read":
		ref := args[len(args)-1]
		_, title, _ := parseRef(ref)
		v, ok := f.items[title]
		if !ok {
			return nil, notFound(title)
		}
		return []byte(v), nil

	case len(args) >= 2 && args[0] == "item" && args[1] == "create":
		title, value, err := parseCreateTemplate(stdin)
		if err != nil {
			return nil, &opCmdError{err: errors.New("exit status 1"), stderr: err.Error()}
		}
		f.items[title] = value
		return []byte(`{"id":"fake"}`), nil

	case len(args) >= 3 && args[0] == "item" && args[1] == "delete":
		title := args[2]
		if _, ok := f.items[title]; !ok {
			return nil, notFound(title)
		}
		delete(f.items, title)
		return nil, nil
	}
	return nil, fmt.Errorf("fakeOP: unexpected args %v", args)
}

// parseRef splits an op://vault/title/field reference.
func parseRef(ref string) (vault, title, field string) {
	parts := strings.SplitN(strings.TrimPrefix(ref, "op://"), "/", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

func parseCreateTemplate(stdin []byte) (title, value string, err error) {
	var tmpl struct {
		Title  string `json:"title"`
		Fields []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(stdin, &tmpl); err != nil {
		return "", "", err
	}
	for _, fld := range tmpl.Fields {
		if fld.Label == opField {
			return tmpl.Title, fld.Value, nil
		}
	}
	return "", "", errors.New("template missing credential field")
}

func newTestOPBackend(f *fakeOP) *opBackend {
	return &opBackend{run: f.run, vault: "Test"}
}

func TestOPBackendRoundTrip(t *testing.T) {
	f := newFakeOP()
	b := newTestOPBackend(f)

	if _, ok, err := b.get("github.com"); err != nil || ok {
		t.Fatalf("get on empty backend: ok=%v err=%v, want false/nil", ok, err)
	}

	want := []byte(`{"token":"github_pat_x","login":"octocat"}`)
	if err := b.set("github.com", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := b.get("github.com")
	if err != nil || !ok {
		t.Fatalf("get after set: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("round-trip data = %q, want %q", got, want)
	}

	if err := b.delete("github.com"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := b.get("github.com"); ok {
		t.Error("item still present after delete")
	}
}

func TestOPBackendDeleteMissingIsNotError(t *testing.T) {
	b := newTestOPBackend(newFakeOP())
	if err := b.delete("github.com"); err != nil {
		t.Errorf("delete of missing item = %v, want nil", err)
	}
}

func TestOPBackendSetFeedsSecretViaStdinNotArgv(t *testing.T) {
	f := newFakeOP()
	b := newTestOPBackend(f)
	// The token marker must reach `op` via stdin and never via argv. (It is the
	// sensitive substring of the stored JSON record; JSON escaping leaves it
	// intact, so we assert on the marker rather than the whole escaped blob.)
	const marker = "github_pat_super_secret"
	if err := b.set("github.com", []byte(`{"token":"`+marker+`"}`)); err != nil {
		t.Fatal(err)
	}

	var createSeen bool
	for _, c := range f.calls {
		for _, a := range c.args {
			if strings.Contains(a, marker) {
				t.Errorf("secret leaked into argv: %q", a)
			}
		}
		if len(c.args) >= 2 && c.args[0] == "item" && c.args[1] == "create" {
			createSeen = true
			if !bytes.Contains(c.stdin, []byte(marker)) {
				t.Errorf("create stdin = %q, want it to contain the secret marker", c.stdin)
			}
		}
	}
	if !createSeen {
		t.Fatal("no `item create` call was made")
	}
}

func TestOPBackendSetDeletesBeforeCreate(t *testing.T) {
	f := newFakeOP()
	b := newTestOPBackend(f)
	if err := b.set("github.com", []byte("v")); err != nil {
		t.Fatal(err)
	}
	var deleteIdx, createIdx = -1, -1
	for i, c := range f.calls {
		switch {
		case len(c.args) >= 2 && c.args[0] == "item" && c.args[1] == "delete":
			deleteIdx = i
		case len(c.args) >= 2 && c.args[0] == "item" && c.args[1] == "create":
			createIdx = i
		}
	}
	if deleteIdx == -1 || createIdx == -1 || deleteIdx > createIdx {
		t.Errorf("expected delete before create, got delete=%d create=%d", deleteIdx, createIdx)
	}
}

func TestOPBackendGetPropagatesRealErrors(t *testing.T) {
	// A non-not-found failure must surface, not be swallowed as "absent".
	b := &opBackend{vault: "Test", run: func([]string, []byte) ([]byte, error) {
		return nil, &opCmdError{err: errors.New("exit status 1"), stderr: "connecting to desktop app: timeout"}
	}}
	if _, ok, err := b.get("github.com"); err == nil || ok {
		t.Errorf("get on transport error: ok=%v err=%v, want false/non-nil", ok, err)
	}
}

func TestOPBackendUsesAccountAndVaultFlags(t *testing.T) {
	f := newFakeOP()
	b := &opBackend{run: f.run, vault: "Work", account: "my-team"}
	if err := b.set("github.com", []byte("v")); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if len(c.args) < 2 || c.args[0] != "--account" || c.args[1] != "my-team" {
			t.Errorf("call missing leading --account my-team: %v", c.args)
		}
		if (contains(c.args, "create") || contains(c.args, "delete")) && !flagValue(c.args, "--vault", "Work") {
			t.Errorf("item command missing --vault Work: %v", c.args)
		}
	}
}

func TestOnePasswordRequested(t *testing.T) {
	cases := map[string]bool{
		"":           false,
		"keychain":   false,
		"1password":  true,
		"op":         true,
		" 1Password": true,
		"OP":         true,
	}
	for val, want := range cases {
		t.Setenv(EnvStore, val)
		if got := onePasswordRequested(); got != want {
			t.Errorf("onePasswordRequested() with %q = %v, want %v", val, got, want)
		}
	}
}

func TestResolveVault(t *testing.T) {
	// No flag, no env → default.
	t.Setenv(EnvVault, "")
	if got := resolveVault(""); got != defaultVault {
		t.Errorf("resolveVault(\"\") with no env = %q, want %q", got, defaultVault)
	}
	// Env used when no flag (and trimmed).
	t.Setenv(EnvVault, "  Engineering ")
	if got := resolveVault(""); got != "Engineering" {
		t.Errorf("resolveVault(\"\") with env = %q, want %q", got, "Engineering")
	}
	// Flag overrides env (and is trimmed).
	if got := resolveVault("  Work "); got != "Work" {
		t.Errorf("resolveVault(flag) = %q, want %q (flag must override env)", got, "Work")
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func flagValue(ss []string, flag, value string) bool {
	for i := 0; i+1 < len(ss); i++ {
		if ss[i] == flag && ss[i+1] == value {
			return true
		}
	}
	return false
}
