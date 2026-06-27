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
	encryptValuePublicKey string
	encryptValueSecretDir string
	encryptValueKeyHint   string
	encryptValueDryRun    bool
)

func init() {
	cmd := encryptValueCmd
	cmd.Flags().StringVarP(&encryptValuePublicKey, "public-key", "k", "", "age public key (or set VAULT_PUBLIC_KEY env)")
	cmd.Flags().StringVarP(&encryptValueSecretDir, "secret-dir", "d", ".vault", "directory to store encrypted secrets")
	cmd.Flags().StringVarP(&encryptValueKeyHint, "key-hint", "n", "VALUE", "hint for placeholder key name (e.g., db_password)")
	cmd.Flags().BoolVar(&encryptValueDryRun, "dry-run", false, "show what would be done without writing")
	rootCmd.AddCommand(cmd)
}

var encryptValueCmd = &cobra.Command{
	Use:     "encrypt-value [value]",
	Aliases: []string{"ev"},
	Short:   "Encrypt a single value and store in .vault/",
	Long: `Encrypt a single literal value and store it in the .vault/ directory.

The value can be provided as a positional argument or piped via stdin.
Outputs the generated placeholder string to stdout.

Examples:
  gitops-vault ev --key-hint db_password "mysecret123"
  echo "mysecret123" | gitops-vault encrypt-value --key-hint db_password`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEncryptValue,
}

func runEncryptValue(cmd *cobra.Command, args []string) error {
	// Read value from arg or stdin
	var value string
	if len(args) > 0 {
		value = args[0]
	} else {
		reader := bufio.NewReader(os.Stdin)
		var lines []string
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return fmt.Errorf("read stdin: %w", err)
			}
			lines = append(lines, line)
			if err == io.EOF {
				break
			}
		}
		value = strings.TrimRight(strings.Join(lines, ""), "\n")
	}

	if value == "" {
		return fmt.Errorf("no value provided: pass as argument or pipe via stdin")
	}

	pubKey, err := vault.LoadPublicKey(encryptValuePublicKey)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}

	store := vault.NewStore(encryptValueSecretDir)
	ph := placeholder.Generate(encryptValueKeyHint)

	fmt.Fprintf(os.Stderr, "Key hint: %s\n", encryptValueKeyHint)
	fmt.Fprintf(os.Stderr, "Placeholder: %s\n", ph)

	if encryptValueDryRun {
		fmt.Println(ph)
		return nil
	}

	encrypted, err := vault.Encrypt(value, pubKey)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := store.Put(ph, encrypted); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Stored in %s/%s\n", encryptValueSecretDir, ph)
	fmt.Println(ph)
	return nil
}
