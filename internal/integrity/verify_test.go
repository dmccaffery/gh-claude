// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package integrity

import (
	"errors"
	"strings"
	"testing"
)

// withExecGH swaps the gh exec stub for the duration of a test.
func withExecGH(t *testing.T, fn func(args ...string) (string, string, error)) {
	t.Helper()
	prev := execGH
	execGH = fn
	t.Cleanup(func() { execGH = prev })
}

func TestVerifyBinaryDefaults(t *testing.T) {
	var got []string
	withExecGH(t, func(args ...string) (string, string, error) {
		got = args
		return "Verification succeeded!", "", nil
	})

	out, err := VerifyBinary("/path/to/gh-claude", VerifyOptions{})
	if err != nil {
		t.Fatalf("VerifyBinary: %v", err)
	}
	if out == "" {
		t.Error("expected gh's report to be returned")
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{"attestation verify /path/to/gh-claude", "--repo " + AttestRepo} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
	// --owner and --repo are mutually exclusive in gh; the default must send only --repo.
	if strings.Contains(joined, "--owner") {
		t.Errorf("args %q must not pass --owner alongside --repo", joined)
	}
}

func TestVerifyBinarySignerWorkflow(t *testing.T) {
	var got []string
	withExecGH(t, func(args ...string) (string, string, error) {
		got = args
		return "ok", "", nil
	})
	opts := VerifyOptions{SignerWorkflow: "o/r/.github/workflows/release.yaml", JSON: true}
	if _, err := VerifyBinary("/bin/x", opts); err != nil {
		t.Fatalf("VerifyBinary: %v", err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--signer-workflow o/r/.github/workflows/release.yaml") {
		t.Errorf("args %q missing signer-workflow", joined)
	}
	if !strings.Contains(joined, "--format json") {
		t.Errorf("args %q missing json format", joined)
	}
}

func TestVerifyBinaryPropagatesFailure(t *testing.T) {
	withExecGH(t, func(args ...string) (string, string, error) {
		return "", "no attestations found", errors.New("exit status 1")
	})
	_, err := VerifyBinary("/bin/x", VerifyOptions{})
	if err == nil {
		t.Fatal("expected error on verification failure")
	}
	if !strings.Contains(err.Error(), "no attestations found") {
		t.Errorf("error %q should include gh's output", err.Error())
	}
}
