## claude completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	claude completion fish | source

To load completions for every new session, execute once:

	claude completion fish > ~/.config/fish/completions/claude.fish

You will need to start a new shell for this setup to take effect.


```
claude completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --op             store the token in 1Password via the op CLI (instead of the OS keychain)
      --vault string   1Password vault for --op (default "Private"; overrides GH_CLAUDE_OP_VAULT)
```

### SEE ALSO

* [claude completion](claude_completion.md)	 - Generate the autocompletion script for the specified shell

