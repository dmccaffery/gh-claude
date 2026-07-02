## claude completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(claude completion zsh)

To load completions for every new session, execute once:

#### Linux:

	claude completion zsh > "${fpath[1]}/_claude"

#### macOS:

	claude completion zsh > $(brew --prefix)/share/zsh/site-functions/_claude

You will need to start a new shell for this setup to take effect.


```
claude completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --op             store the token in 1Password via the op CLI (instead of the OS keychain)
      --vault string   1Password vault for --op (default "Private"; overrides GH_CLAUDE_OP_VAULT)
```

### SEE ALSO

* [claude completion](claude_completion.md)	 - Generate the autocompletion script for the specified shell

