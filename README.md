# gh-claude

A [GitHub CLI](https://cli.github.com) extension that launches [Claude Code](https://claude.com/claude-code)
with a **temporary, least-privilege GitHub token** so Claude can work with your
**private repositories** without ever seeing your real credential.

The token is:

- **read-only on source code** (no `git push`),
- **read/write on issues and pull requests**,
- **scoped to all repositories you can access**,
- **expiring after 7 days**, and
- **stored in your OS keychain** (or an encrypted file where no keychain exists, e.g. WSL2).

Claude only ever receives the scoped token through the `GITHUB_TOKEN` environment
variable — it never touches your keychain or your real `gh` credential.

## Why a manual browser step?

GitHub has **no API to mint a fine-grained personal access token** — they can only
be created in the browser. (The fully-automated alternative, a GitHub App, would
require shipping an app private key to every developer machine, which we explicitly
avoid.) So `gh-claude` automates everything *except* the creation click: it opens
the token page **pre-filled** with the right name, 7-day expiry, and permissions;
you choose "All repositories" and click **Generate**; and from then on the
extension stores, reuses, validates, and wires up the token for you. You only
repeat the browser step about once a week, when the token expires.

## Install

```sh
gh extension install bitwise-media-group/gh-claude
```

Requires the `gh` CLI (authenticated), `git`, and the `claude` CLI on your `PATH`.

## Usage

```sh
# Ensure a valid token (reuse or create), then launch Claude in the current dir:
gh claude

# Pass arguments through to claude after `--`:
gh claude -- --resume

# Force-create a fresh token without launching Claude:
gh claude login

# Show the stored token's account, expiry, and where it is stored:
gh claude status

# Remove the stored token:
gh claude logout

# Force a new token, then launch:
gh claude --refresh
```

On first run (or after expiry) your browser opens to the GitHub token page. In it:

1. Under **Repository access** choose **All repositories**.
2. Leave the pre-filled permissions: **Contents: Read**, **Issues: Read and write**,
   **Pull requests: Read and write**.
3. Click **Generate token** and copy the value (starts with `github_pat_`).
4. Paste it back into the terminal (input is hidden).

The extension validates the token, records its exact expiry (from GitHub's
`GitHub-Authentication-Token-Expiration` response header), stores it, and launches
Claude. Subsequent runs reuse the stored token until it is within 5 minutes of
expiry or has been revoked.

## How gh and git are wired

When launching Claude, `gh-claude` adds to the child process environment only
(your global git config and `gh` keychain entry are never modified):

- `GH_TOKEN` and `GITHUB_TOKEN` — set to the scoped token. `gh` prefers these over
  its keychain, so Claude's `gh` uses the scoped token and never reads the keychain.
- A per-process git credential helper, injected via git's `GIT_CONFIG_*`
  environment mechanism, pointing at `gh auth git-credential`. `git` then defers to
  `gh`, which returns the scoped token — so `git clone`/`fetch` over HTTPS work for
  private repos, while `git push` is rejected (the token is read-only on contents).

## Security model

- **Claude never sees your keychain or real credential.** It receives only the
  scoped, read-only-code token via an environment variable.
- **No push.** The token's `contents: read` scope means code cannot be pushed,
  bounding the blast radius if the token leaks.
- **Short-lived.** Tokens expire after 7 days; reuse stops 5 minutes before expiry.
- **Env-var exposure.** As with any `GITHUB_TOKEN`, the value is readable by your
  own processes (e.g. `/proc/<pid>/environ` on Linux). This is inherent to the
  env-var delivery model; the read-only scope is what limits the risk.
- **WSL2 / no-keychain fallback.** Where no OS keychain is reachable, the token is
  stored in an encrypted file (mode `0600`) under your config directory. The
  encryption key is derived from a stable machine identifier (`/etc/machine-id`);
  this is defense-in-depth, **not** a hardware-backed secret — the machine id is
  not itself secret, so the file's permissions are the primary protection. Run
  `gh claude status` to see which backend is in use.

## Development

```sh
go test ./...        # run the test suite
go build -o gh-claude .
gh extension install .   # install your local build for end-to-end testing
```

Releases are built by `.github/workflows/release.yml` (via
[`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile))
on pushing a `vX.Y.Z` tag.
