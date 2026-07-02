<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Security model

gh-claude exists to hand Claude Code a credential you can afford to lose.
Everything in its design follows from that: the token is minted with the least
privilege that still lets Claude do useful work, it dies young, it is stored in
the strongest secret store the machine has, it is wired into the child process
only — and the binary that does all this can prove where it came from and can be
remotely revoked if a release ever turns out to be bad.

At a glance:

| Claude gets                             | Claude never sees         |
| --------------------------------------- | ------------------------- |
| a scoped token via `GITHUB_TOKEN`       | your OS keychain          |
| `contents: read` (source code, no push) | your real `gh` credential |
| `issues: read/write`                    | `git push` (rejected)     |
| `pull requests: read/write`             | anything past 7 days      |

## The token

### Scope

The extension provisions a **fine-grained personal access token**
(`github_pat_…`) with exactly these permissions, applied to **all repositories
you can access**:

| Permission      | Access         | Why                                        |
| --------------- | -------------- | ------------------------------------------ |
| `contents`      | read           | clone/fetch source; `git push` is rejected |
| `issues`        | read and write | work the backlog                           |
| `pull_requests` | read and write | open and review PRs                        |
| `metadata`      | read           | added automatically by GitHub              |

Classic tokens are refused outright: the paste step checks the `github_pat_`
prefix, because only fine-grained PATs can carry this per-permission shape and
the 7-day expiry.

### Why a browser step at all

GitHub has **no API to mint a fine-grained PAT** — they can only be created in
the browser. The fully-automated alternative, a GitHub App, would mean shipping
an app private key to every developer machine, which this project explicitly
avoids. So gh-claude automates everything _except_ the creation click: it opens
`https://github.com/settings/personal-access-tokens/new` **pre-filled** with the
right name, description, 7-day expiry, and permissions; you choose "All
repositories" and click **Generate**; the extension does the rest. The browser
comes round roughly once a week.

The pre-filled name is `gh-claude (<host> <DDMMYYYY>)` — the machine's short
hostname plus the creation date, e.g. `gh-claude (mac 02072026)`. Fine-grained
PAT names are **unique per user**, and a renewal usually happens while the
previous token is still live, so the date stamp is what keeps the new token's
name from colliding with the one it replaces.

### Lifecycle

- **Validated on paste.** The extension calls GitHub with the new token and
  reads the exact expiry from the `GitHub-Authentication-Token-Expiration`
  response header. A token that lives longer than 7 days (plus a 12-hour
  clock-skew tolerance) is rejected — a long-lived paste cannot sneak in.
- **Reused until 5 minutes before expiry.** Within that buffer, or when GitHub
  actively rejects the token (revoked), a fresh one is provisioned. Within 24
  hours of expiry you are warned that the browser step is coming.
- **Trusted when offline.** If GitHub simply cannot be reached to re-validate,
  the stored token is used anyway — transient network failure does not lock you
  out.

## Where the token lives

The strongest store available wins; `gh claude status` shows which backend is in
use.

### macOS Keychain

Stored as a generic password (service `gh-claude`) in the login Keychain via the
built-in `/usr/bin/security` tool — standard library only, no cgo, no
third-party keyring, so it works identically in the release binaries. The
backend is probed at startup; if the keychain is unreachable the encrypted-file
fallback is used (with a warning, since on macOS that is unexpected).

### Windows Credential Manager

A generic credential named `gh-claude:github.com`, written through `advapi32`
(`CredWriteW`/`CredReadW`/`CredDeleteW`) — again pure standard library, no cgo.

### Encrypted file (Linux, WSL2, everything else)

Hosts without a native keychain get an encrypted file under the gh-claude config
directory (`$XDG_CONFIG_HOME/gh-claude` or `~/.config/gh-claude`) — this is the
_standard_ backend there, not an error condition:

- **AES-256-GCM**, per-item random salt and nonce, with the format version and
  key slot bound into the authenticated data.
- The key is derived with **HKDF-SHA256** from a stable machine identifier
  (`/etc/machine-id` or `/var/lib/dbus/machine-id`), or from a generated
  per-install key (mode `0600`) where none exists.
- The directory is `0700`, the files `0600`.

!!! warning "Defense in depth, not hardware"

    The machine id is not itself a secret, so the encryption is
    defense-in-depth against casual file exposure (backups, copied home
    directories) — the file permissions are the primary protection. If you
    want a real secret store on such hosts, use the 1Password backend below.

A blob that no longer decrypts is deleted and you are asked to re-authenticate
with `gh claude login` — the store fails safe, never half-open.

### 1Password (opt-in)

With `--op` (or `GH_CLAUDE_STORE=1password`), the token is kept as a single
concealed field of an _API Credential_ item named `gh-claude:github.com`, via
the [1Password CLI](https://developer.1password.com/docs/cli/). The vault comes
from `--vault`, then `GH_CLAUDE_OP_VAULT`, then the default `Private`;
`GH_CLAUDE_OP_ACCOUNT` selects an account in multi-account setups. The value
only ever crosses to `op` over **stdin** — never on a command line — and reads
prompt for 1Password's own approval (biometric/system unlock). If 1Password is
requested but `op` is missing or not signed in, gh-claude **fails with a clear
error** rather than silently falling back to a weaker store.

## How Claude is launched

Launch wiring is **process-local and ephemeral** — no file on disk is modified,
your global git config and `gh` keychain entry are untouched:

- `GH_TOKEN` and `GITHUB_TOKEN` are set to the scoped token in the child
  environment. `gh` prefers these over its keychain, so Claude's `gh` never
  reads your real credential.
- A per-process git credential helper is injected through git's `GIT_CONFIG_*`
  environment mechanism: any inherited helper list is reset, then
  `!gh auth git-credential` is installed for `https://github.com`. HTTPS clone
  and fetch work on private repos; `git push` is rejected by the token's
  read-only contents scope.
- On macOS and Linux the process is **replaced** by `claude` (`execve`), so
  signals, the terminal, and the exit code pass straight through. Windows has no
  exec replacement, so a child process is spawned with inherited stdio and its
  exit code is propagated.

!!! note "Environment variables are readable by your own processes"

    As with any `GITHUB_TOKEN`, the value is visible to processes running as
    you (e.g. `/proc/<pid>/environ` on Linux). That is inherent to env-var
    delivery — the read-only scope and 7-day expiry are what bound the risk.

## Binary integrity

gh-claude ships **two independent integrity mechanisms** with different trust
roots and cadences, on purpose:

|                     | Build provenance                      | Security policy                                               |
| ------------------- | ------------------------------------- | ------------------------------------------------------------- |
| Question it answers | "Did our CI build this exact binary?" | "Is this version known-bad right now?"                        |
| Trust root          | GitHub Sigstore attestation store     | Embedded OpenSSH policy keys — Ed25519, primary + backup      |
| When it runs        | On demand (`gh claude verify`)        | On launch (`gh claude`, `gh claude login`), throttled         |
| Needs network       | Yes                                   | Only to refresh a stale cache                                 |
| Failure mode        | Reports the error                     | **Fail-open** (warn) unless a trusted policy says **blocked** |

Why two roots? A Sigstore-keyless signature **cannot be revoked** — Fulcio
certificates are short-lived and Rekor inclusion proofs are permanent, so a
signature that verified yesterday verifies forever. Revocation therefore has to
be _additive_: a separately-signed policy that **names** bad versions and that
the client consults. Keeping the policy key independent of the release-signing
identity means a revocation can ship **without cutting a new binary release**,
and the policy stays trustworthy even if the release path is compromised.

### Build provenance — `gh claude verify`

Verifies the running binary against GitHub's attestation store using
`gh attestation verify` (the `gh` CLI is always present — this is a gh extension
— and carries Sigstore's trusted root):

```console
$ gh claude verify                 # asserts: built by our CI from this repo
$ gh claude verify --signer-workflow \
    bitwise-media-group/github-workflows/.github/workflows/release.yaml
$ gh claude verify --json          # machine-readable gh output
```

No setup needed — the release workflow already emits the SLSA build-provenance
attestation. The same check works on a downloaded binary before you install it;
see [verifying the artifacts](install.md#verifying-the-artifacts).

### The security policy — a revocable kill switch

If a released version is found to leak the token, or a tampered artifact is seen
in the wild, the fix must reach users _faster_ than a release can. That is the
policy system's job: a small signed JSON document, published alongside this
site, that the client checks on launch and that can **refuse to launch** a
known-bad build.

#### The policy document

The policy is served at `https://oss.bitwisemedia.uk/gh-claude/policy.json`
(override with `GH_CLAUDE_POLICY_URL` for self-hosting or testing), with a
detached OpenSSH signature at the same URL plus `.sig`:

```json
{
  "schema": 1,
  "sequence": 7,
  "issued_at": "2026-07-01T00:00:00Z",
  "expires_at": "2026-07-15T00:00:00Z",
  "min_safe_version": "1.4.0",
  "revoked": [
    {
      "version": "1.3.0",
      "reason": "CVE-2026-1234: scoped token written to a world-readable log",
      "advisory": "https://github.com/bitwise-media-group/gh-claude/security/advisories/GHSA-xxxx-xxxx-xxxx"
    },
    { "digest": "sha256:2f0c…", "reason": "tampered artifact seen in the wild" }
  ]
}
```

Field by field:

- **`schema`** — the document format version; the client rejects unknown schemas
  rather than guessing.
- **`sequence`** — strictly **monotonic**. A fetched policy with a lower
  sequence than the cached one is rejected as a rollback: an attacker cannot
  "un-revoke" a version by replaying an older, pre-revocation policy they
  captured earlier.
- **`issued_at` / `expires_at`** — the freshness window. Past `expires_at`, a
  cached "all clear" is no longer trusted until a newer policy is fetched — this
  defeats a _freeze attack_, where an attacker blocks the client's view of new
  policies and serves it a stale clean one forever. Expiry does **not** discard
  a _block_: a known-bad verdict stands even when stale or offline.
- **`min_safe_version`** — a version floor. Every release below it is blocked
  (semver comparison, leading `v` optional), so one field revokes a whole range
  of vulnerable versions at once.
- **`revoked[]`** — precise kills: an exact `version`, and/or an artifact
  `digest` (`sha256:<hex>` of the running binary — this catches a tampered build
  whose _version string_ is innocent). Each entry carries a human `reason` and
  an `advisory` URL, both shown to the user in the block message.

#### How it is fetched and verified

On launch (`gh claude` and `gh claude login`) the client:

1. **Loads the cached policy** from `security-policy.json` under the gh-claude
   config dir — and re-verifies its signature; a tampered cache is treated as
   absent.
2. **Refreshes over the network** only when needed: no cache, cache older than
   **6 hours**, or cache past its `expires_at`. Fetches are bounded — a 3-second
   timeout per request and a 1 MiB response ceiling — so the check can never
   hang a launch.
3. **Verifies the signature** before believing anything. Policies are signed in
   the OpenSSH signature format (`ssh-keygen -Y sign`, the
   `-----BEGIN SSH SIGNATURE-----` "sshsig" blob) under the fixed namespace
   `gh-claude-policy`. The client accepts a signature made by **either** of two
   Ed25519 public keys **embedded in the binary** — a primary and a break-glass
   backup, held apart, so a lost or compromised primary never locks the channel.
   The keys can be FIDO2-resident `sk-ssh-ed25519` keys on YubiKeys: the
   signature then carries the authenticator's flags-and-counter envelope, which
   the client verifies too. Nothing secret ships in the binary — public keys
   only.
4. **Enforces monotonicity**: a fetched policy whose `sequence` is lower than
   the cached one is discarded, and the cache is kept.
5. **Evaluates the running build**: digest match, then exact version match, then
   the `min_safe_version` floor. The binary is only hashed when the policy
   actually contains digest rules.

#### What happens on each outcome

| Situation                                            | Verdict     | Launch                                                 |
| ---------------------------------------------------- | ----------- | ------------------------------------------------------ |
| Trusted policy, version fine                         | clear       | proceeds silently                                      |
| Trusted policy revokes version/digest or below floor | **blocked** | **refused** (fail-closed) with advisory + upgrade hint |
| No policy reachable, none cached                     | unknown     | proceeds with a stderr warning (fail-open)             |
| Cached "all clear" expired, refresh failed           | unknown     | proceeds with a staleness warning                      |
| Cached policy blocks, network down                   | **blocked** | **refused** — offline never un-revokes                 |

The asymmetry is deliberate. The channel **fails open** on absence — a broken
CDN or an air-gapped machine must not brick every installation — but **fails
closed** on knowledge: once any trusted policy has said "this version is bad",
that verdict is cached and going offline does not lift it. A blocked launch
prints the `reason` and `advisory` from the policy and points at the fix:

```console
$ gh claude
error: gh-claude 1.3.0 is blocked by the security policy: CVE-2026-1234: …
Advisory: https://github.com/bitwise-media-group/gh-claude/security/advisories/GHSA-xxxx-xxxx-xxxx
Upgrade with: gh extension upgrade claude
```

#### Escape hatches and edges

| Toggle / condition         | Effect                                                               |
| -------------------------- | -------------------------------------------------------------------- |
| `GH_CLAUDE_SKIP_INTEGRITY` | Any non-empty value disables the launch-time check (air-gapped use). |
| `GH_CLAUDE_POLICY_URL`     | Override the policy endpoint (self-hosting / testing).               |
| Local / snapshot builds    | Report a non-release version and are never gated.                    |
| Unconfigured builds        | With no embedded key and no URL the check is a silent no-op.         |

How policies are authored, signed with a YubiKey, published, and rotated —
including the disclosure runbook — is covered in
[Policy operations](security-policy.md).
