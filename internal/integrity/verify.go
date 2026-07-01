// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package integrity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// Defaults describing who is allowed to have built a genuine gh-claude binary.
// These are the assertions `gh claude verify` makes against GitHub's Sigstore
// attestation store.
const (
	// AttestOwner is the org whose attestations are trusted.
	AttestOwner = "bitwise-media-group"
	// AttestRepo is the source repository the build provenance must name.
	AttestRepo = "bitwise-media-group/gh-claude"
)

// VerifyOptions tunes the provenance assertion. The zero value verifies against
// AttestRepo, which is the minimal honest claim ("built by this org's CI from
// this repo"); SignerWorkflow tightens it to an exact build path. Owner is an
// alternative, looser scope (any repo under the org) and is mutually exclusive
// with Repo — gh rejects passing both — so Repo wins when both are set.
type VerifyOptions struct {
	Owner          string // org scope; used only when Repo is empty
	Repo           string // "owner/repo"; defaults to AttestRepo, and takes precedence over Owner
	SignerWorkflow string // e.g. "bitwise-media-group/github-workflows/.github/workflows/release.yaml"
	CertIdentity   string // regexp alternative to SignerWorkflow
	JSON           bool   // request --format json instead of the human summary
}

// execGH runs the gh CLI and returns combined stdout/stderr. It is a package
// variable so tests can stub the subprocess. gh is always on PATH here — this is
// a gh extension — and it ships Sigstore's trusted root, so verification needs no
// extra dependency and can run offline against a bundled root.
var execGH = func(args ...string) (stdout, stderr string, err error) {
	so, se, err := gh.Exec(args...)
	return so.String(), se.String(), err
}

// VerifyBinary asserts, via `gh attestation verify`, that the artifact at path
// was built by the expected repository's workflow and recorded in GitHub's
// attestation store. It returns gh's report (stdout, or stderr when gh writes the
// summary there) on success, and a wrapped error including gh's output on
// failure — a verification failure is a security signal the caller should show
// verbatim.
func VerifyBinary(path string, opts VerifyOptions) (string, error) {
	// gh treats --owner and --repo as mutually exclusive; --repo is the stricter
	// claim, so prefer it and only fall back to --owner when no repo is given.
	args := []string{"attestation", "verify", path}
	switch {
	case opts.Repo != "":
		args = append(args, "--repo", opts.Repo)
	case opts.Owner != "":
		args = append(args, "--owner", opts.Owner)
	default:
		args = append(args, "--repo", AttestRepo)
	}
	switch {
	case opts.SignerWorkflow != "":
		args = append(args, "--signer-workflow", opts.SignerWorkflow)
	case opts.CertIdentity != "":
		args = append(args, "--cert-identity-regex", opts.CertIdentity)
	}
	if opts.JSON {
		args = append(args, "--format", "json")
	}

	stdout, stderr, err := execGH(args...)
	if err != nil {
		return "", fmt.Errorf("attestation verification failed: %w\n%s", err, strings.TrimSpace(stderr+"\n"+stdout))
	}
	out := strings.TrimSpace(stdout)
	if out == "" {
		out = strings.TrimSpace(stderr) // gh prints its human summary to stderr
	}
	return out, nil
}

// RunningBinaryPath returns the absolute, symlink-resolved path of the running
// executable — the artifact `gh claude verify` should check.
func RunningBinaryPath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Abs(path)
}
