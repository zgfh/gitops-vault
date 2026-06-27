package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gitops-vault",
	Short: "A secret desensitization tool for GitOps repositories",
	Long: `gitops-vault is a GitOps-friendly secret management tool that uses
text placeholder replacement to encrypt sensitive values in GitOps repos.

Sensitive values are replaced with VAULT_ placeholders and encrypted into the
.vault/ directory. Both files and .vault/ can be committed to git. At deploy
time, placeholders are transparently restored via decryption.`,
}

func init() {
	// Override help to show aliases next to command names.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printHelp(cmd)
	})
}

func printHelp(cmd *cobra.Command) {
	out := cmd.OutOrStdout()

	if cmd.Long != "" {
		fmt.Fprintln(out, strings.TrimRight(cmd.Long, "\n"))
	} else if cmd.Short != "" {
		fmt.Fprintln(out, cmd.Short)
	}
	fmt.Fprintln(out)

	subs := cmd.Commands()
	isLeaf := len(subs) == 0

	if isLeaf {
		fmt.Fprintf(out, "Usage:\n  %s\n\n", cmd.UseLine())
	} else {
		fmt.Fprintf(out, "Usage:\n  %s [command]\n\n", cmd.CommandPath())
	}

	if len(subs) > 0 {
		fmt.Fprintln(out, "Available Commands:")
		for _, sub := range subs {
			if !sub.IsAvailableCommand() || sub.Name() == "help" {
				continue
			}
			line := fmt.Sprintf("  %-14s", sub.Name())
			if len(sub.Aliases) > 0 {
				line += fmt.Sprintf("(%s)", strings.Join(sub.Aliases, ", "))
			}
			prefixLen := len(line)
			if prefixLen < 28 {
				line += strings.Repeat(" ", 28-prefixLen)
			} else {
				line += " "
			}
			line += sub.Short
			fmt.Fprintln(out, line)
		}
		fmt.Fprintln(out)
	}

	localFlags := cmd.LocalFlags()
	if localFlags.HasFlags() {
		fmt.Fprintln(out, "Flags:")
		fmt.Fprint(out, localFlags.FlagUsages())
		fmt.Fprintln(out)
	}

	if !isLeaf {
		fmt.Fprintf(out, `Use "%s [command] --help" for more information about a command.`+"\n", cmd.CommandPath())
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
