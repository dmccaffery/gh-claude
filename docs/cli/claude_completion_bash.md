## claude completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(claude completion bash)

To load completions for every new session, execute once:

#### Linux:

	claude completion bash > /etc/bash_completion.d/claude

#### macOS:

	claude completion bash > $(brew --prefix)/etc/bash_completion.d/claude

You will need to start a new shell for this setup to take effect.


```
claude completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --op             store the token in 1Password via the op CLI (instead of the OS keychain)
      --vault string   1Password vault for --op (default "Private"; overrides GH_CLAUDE_OP_VAULT)
```

### SEE ALSO

* [claude completion](claude_completion.md)	 - Generate the autocompletion script for the specified shell

