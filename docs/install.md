<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Installation

gh-claude installs as a [GitHub CLI](https://cli.github.com) extension — one
command, upgrades included. Every release ships raw per-platform binaries with
checksums, keyless [cosign](#signatures) signatures, SPDX [SBOMs](#sboms), and a
SLSA build-provenance [attestation](#attestations) you can verify yourself.

## GitHub CLI extension

Requires the `gh` CLI (authenticated), `git`, and the `claude` CLI on your
`PATH`:

```sh
gh extension install bitwise-media-group/gh-claude
```

Upgrade and uninstall the usual way:

```sh
gh extension upgrade claude
gh extension remove claude
```

!!! note "Why the release assets are raw binaries"

    `gh extension install` picks the release asset whose name matches
    `gh-claude-<os>-<arch>[.exe]` — raw binaries, not archives. That naming is
    what makes the extension installable and upgradeable through `gh`, so it is
    also the naming used in the verification steps below. `<os>` is `darwin`,
    `linux`, or `windows`; `<arch>` is `amd64` or `arm64`.

## From source

With the repository cloned and the Go toolchain installed, `make install` builds
the binary and installs the working copy as the `claude` extension:

```sh
git clone https://github.com/bitwise-media-group/gh-claude
cd gh-claude
make install
```

!!! note "Source builds are unstamped"

    A local build carries no release version stamp, is not covered by the
    cosign signature or attestation below, and is never gated by the
    [security policy](security.md#the-security-policy-a-revocable-kill-switch).
    Install the released extension when you want a verifiable artifact.

## Verifying the artifacts

Every release attaches a `checksums.txt`, a cosign signature bundle per binary,
SPDX SBOMs, and a GitHub build-provenance attestation. None of the steps below
require trusting a long-lived key — cosign and `gh` verify against Sigstore's
transparency log and GitHub's attestation API.

### Checksums

`checksums.txt` lists the SHA-256 of every release binary. Download it alongside
the binaries you grabbed and check them:

```sh
sha256sum --ignore-missing -c checksums.txt
```

On macOS without the GNU coreutils, use
`shasum -a 256 --ignore-missing -c checksums.txt`.

### Signatures

Each binary is signed keyless with [cosign](https://docs.sigstore.dev/) in the
release workflow; the signature travels as a Sigstore bundle named
`gh-claude_<os>_<arch>.sigstore.json` on the release. Download the binary and
its matching bundle, then verify one against the other:

```sh
cosign verify-blob \
  --certificate-identity-regexp '^https://github.com/bitwise-media-group/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle gh-claude_darwin_arm64.sigstore.json \
  gh-claude-darwin-arm64
```

!!! info "Why a regexp for the identity"

    gh-claude signs from the organisation's
    [reusable release workflow](https://github.com/bitwise-media-group/github-workflows),
    which is pinned — and bumped — by commit SHA, so the certificate's exact
    identity URL changes between releases. The
    `^https://github.com/bitwise-media-group/` regexp pins the signer to the
    organisation while staying stable across those bumps. The OIDC issuer is
    always GitHub Actions, so it is matched exactly.

### Attestations

The release workflow records a
[SLSA build-provenance attestation](https://docs.github.com/en/actions/security-guides/using-artifact-attestations)
over everything in `checksums.txt`. Verify a downloaded binary with the GitHub
CLI — no download of the attestation needed, `gh` fetches it from the API:

```sh
gh attestation verify gh-claude-darwin-arm64 \
  --repo bitwise-media-group/gh-claude
```

Once installed, the extension can also verify **itself** — `gh claude verify`
runs the same check against the running binary. See the
[security model](security.md#build-provenance-gh-claude-verify).

### SBOMs

An SPDX software bill of materials is attached for each binary as
`gh-claude-<os>-<arch>.sbom.json`. Inspect it with any SPDX-aware tool, for
example:

```sh
grype sbom:gh-claude-darwin-arm64.sbom.json
```
