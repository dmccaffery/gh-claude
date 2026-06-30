// Command gh-claude is a GitHub CLI extension that launches Claude Code with a
// temporary, least-privilege GitHub token (read-only on source code, read/write
// on issues and pull requests) so Claude can work with private repositories
// without ever seeing the user's real credential or the OS keychain.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bitwise-media-group/gh-claude/internal/browser"
	"github.com/bitwise-media-group/gh-claude/internal/launch"
	"github.com/bitwise-media-group/gh-claude/internal/store"
	"github.com/bitwise-media-group/gh-claude/internal/token"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var refresh bool

	root := &cobra.Command{
		Use:   "claude [-- claude-args...]",
		Short: "Launch Claude Code with a temporary, no-push GitHub token",
		Long: `Launch Claude Code with a temporary, least-privilege GitHub token.

The token is read-only on source code (no push) and read/write on issues and
pull requests, expires in 7 days, and is stored in your OS keychain (or an
encrypted file on systems without one, such as WSL2). An unexpired token is
reused; a new one is created in your browser only when needed. Claude is then
launched in the current directory with the token wired into gh and git.

Pass arguments through to claude after "--", e.g.:
  gh claude -- --resume`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunch(refresh, passthroughArgs(cmd, args))
		},
	}
	root.Flags().BoolVar(&refresh, "refresh", false, "force creating a new token even if a valid one is stored")

	root.AddCommand(loginCmd(), logoutCmd(), statusCmd())
	return root
}

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "login",
		Short:         "Create or refresh the stored token without launching Claude",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			mgr, st, err := newManager()
			if err != nil {
				return err
			}
			warnIfFileFallback(st)
			rec, err := mgr.Ensure(true, newProvisioner())
			if err != nil {
				return err
			}
			fmt.Printf("Logged in as @%s. Token expires %s.\n", rec.Login, rec.ExpiresAt.Format(time.RFC1123))
			return nil
		},
	}
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "logout",
		Short:         "Remove the stored token",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			mgr, _, err := newManager()
			if err != nil {
				return err
			}
			if err := mgr.Clear(); err != nil {
				return err
			}
			fmt.Println("Removed the stored token.")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "status",
		Short:         "Show the stored token's account and expiry",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			mgr, st, err := newManager()
			if err != nil {
				return err
			}
			rec, err := mgr.Current()
			if err != nil {
				return err
			}
			fmt.Printf("Storage:  %s\n", st.Backend())
			if rec == nil {
				fmt.Println("Token:    none stored — run `gh claude` to create one")
				return nil
			}
			fmt.Printf("Account:  @%s\n", rec.Login)
			fmt.Printf("Host:     %s\n", rec.Host)
			fmt.Printf("Created:  %s\n", rec.CreatedAt.Format(time.RFC1123))
			remaining := time.Until(rec.ExpiresAt)
			if remaining <= 0 {
				fmt.Printf("Expires:  %s (EXPIRED — a new token will be created on next launch)\n",
					rec.ExpiresAt.Format(time.RFC1123))
			} else {
				fmt.Printf("Expires:  %s (%s remaining)\n",
					rec.ExpiresAt.Format(time.RFC1123), humanizeDuration(remaining))
			}
			return nil
		},
	}
}

// passthroughArgs returns the args to forward to claude: everything after a
// "--" separator, or all positional args when no separator was used.
func passthroughArgs(cmd *cobra.Command, args []string) []string {
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		return args[dash:]
	}
	return args
}

func runLaunch(refresh bool, claudeArgs []string) error {
	mgr, st, err := newManager()
	if err != nil {
		return err
	}
	warnIfFileFallback(st)
	rec, err := mgr.Ensure(refresh, newProvisioner())
	if err != nil {
		return err
	}
	// Replaces this process with claude on success.
	return launch.Run(rec.Token, claudeArgs)
}

func newManager() (*token.Manager, *store.Store, error) {
	st, err := store.New()
	if err != nil {
		return nil, nil, err
	}
	return &token.Manager{Store: st, Out: os.Stderr}, st, nil
}

func newProvisioner() token.Provisioner {
	hostname, _ := os.Hostname()
	return token.Provisioner{
		Hostname:  hostname,
		OpenURL:   browser.Open,
		ReadToken: func() (string, error) { return readToken(os.Stdin, os.Stderr) },
	}
}

func warnIfFileFallback(st *store.Store) {
	if st.IsFileFallback() {
		fmt.Fprintln(os.Stderr, "warning: no OS keychain available; storing the token in an encrypted file (see README \"Security model\").")
	}
}

// readToken prompts for and reads the pasted token, hiding input on a terminal.
func readToken(in *os.File, out io.Writer) (string, error) {
	fmt.Fprint(out, "Paste the new token (input hidden), then press Enter: ")
	defer fmt.Fprintln(out)

	fd := int(in.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		return string(b), err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

// humanizeDuration renders a positive duration as a compact "Xd Yh Zm" string.
func humanizeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
