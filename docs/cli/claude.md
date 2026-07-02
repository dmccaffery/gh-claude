## claude

Launch Claude Code with a temporary, no-push GitHub token

### Synopsis

Launch Claude Code with a temporary, least-privilege GitHub token.

The token is read-only on source code (no push) and read/write on issues and
pull requests, expires in 7 days, and is stored in your OS keychain (or an
encrypted file on systems without one, such as WSL2). An unexpired token is
reused; a new one is created in your browser only when needed. Claude is then
launched in the current directory with the token wired into gh and git.

Use --op to store the token in 1Password (via the op CLI) instead of the OS
keychain, and --vault to choose the vault.

Pass arguments through to claude after "--", e.g.:
  gh claude -- --resume

```
claude [-- claude-args...] [flags]
```

### Options

```
  -h, --help           help for claude
      --op             store the token in 1Password via the op CLI (instead of the OS keychain)
      --refresh        force creating a new token even if a valid one is stored
      --vault string   1Password vault for --op (default "Private"; overrides GH_CLAUDE_OP_VAULT)
```

### SEE ALSO

* [claude completion](claude_completion.md)	 - Generate the autocompletion script for the specified shell
* [claude login](claude_login.md)	 - Create or refresh the stored token without launching Claude
* [claude logout](claude_logout.md)	 - Remove the stored token
* [claude status](claude_status.md)	 - Show the stored token's account and expiry
* [claude verify](claude_verify.md)	 - Verify this binary's build provenance via GitHub attestations
* [claude version](claude_version.md)	 - Print the gh-claude version

