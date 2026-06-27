package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zzg/gitops-vault/pkg/placeholder"
	"github.com/zzg/gitops-vault/pkg/vault"
)

var (
	decryptValuePrivateKey string
	decryptValueSecretDir  string
	decryptValueInput      string
)

func init() {
	cmd := decryptValueCmd
	cmd.Flags().StringVarP(&decryptValuePrivateKey, "private-key", "k", "", "age private key (or set AGE_KEY env)")
	cmd.Flags().StringVarP(&decryptValueSecretDir, "secret-dir", "d", ".vault", "directory containing encrypted secrets")
	cmd.Flags().StringVarP(&decryptValueInput, "input", "i", "", "base64-encoded encrypted blob (instead of looking up by placeholder)")
	rootCmd.AddCommand(cmd)
}

var decryptValueCmd = &cobra.Command{
	Use:     "decrypt-value [placeholder]",
	Aliases: []string{"dv"},
	Short:   "Decrypt a single value from .vault/ or stdin",
	Long: `Decrypt a single value from the .vault/ directory.

The placeholder can be provided as a positional argument or piped via stdin.
Alternatively, use --input to directly decrypt a base64-encoded age blob.

Outputs the decrypted plaintext to stdout.

Examples:
  gitops-vault decrypt-value VAULT_DB_PASSWORD_1719460000
  echo "VAULT_DB_PASSWORD_1719460000" | gitops-vault decrypt-value
  gitops-vault decrypt-value --input "YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxO..."`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDecryptValue,
}

func runDecryptValue(cmd *cobra.Command, args []string) error {
	privKey, err := vault.LoadPrivateKey(decryptValuePrivateKey)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	sd := decryptValueSecretDir
	store := vault.NewStore(sd)

	// --input mode: direct base64 blob
	if decryptValueInput != "" {
		plaintext, err := vault.Decrypt(decryptValueInput, privKey)
		if err != nil {
			return fmt.Errorf("decrypt: %w", err)
		}
		fmt.Print(plaintext)
		return nil
	}

	// Get placeholder from arg or stdin
	var ph string
	if len(args) > 0 {
		ph = strings.TrimSpace(args[0])
	} else {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read stdin: %w", err)
		}
		ph = strings.TrimSpace(line)
	}

	if ph == "" {
		return fmt.Errorf("no placeholder provided: pass as argument or pipe via stdin")
	}

	if !placeholder.IsPlaceholder(ph) {
		return fmt.Errorf("not a valid placeholder: %s", ph)
	}

	encrypted, err := store.Get(ph)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", ph, err)
	}

	plaintext, err := vault.Decrypt(encrypted, privKey)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	fmt.Print(plaintext)
	return nil
}
