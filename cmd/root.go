package cmd

import (
	"fmt"
	"os"

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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
