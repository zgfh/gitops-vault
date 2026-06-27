package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zzg/gitops-vault/pkg/config"
	"github.com/zzg/gitops-vault/pkg/placeholder"
	"github.com/zzg/gitops-vault/pkg/vault"
)

var (
	evPublicKey string
	evSecretDir string
	evKeyName   string
)

func init() {
	encryptValueCmd.Flags().StringVarP(&evPublicKey, "public-key", "k", "", "age public key (or use config/VAULT_PUBLIC_KEY)")
	encryptValueCmd.Flags().StringVarP(&evSecretDir, "secret-dir", "d", "", "directory to store encrypted secrets (default from config or .vault)")
	encryptValueCmd.Flags().StringVarP(&evKeyName, "key", "n", "", "key name for the placeholder (e.g., db_password)")
	encryptValueCmd.MarkFlagRequired("key")
	rootCmd.AddCommand(encryptValueCmd)
}

var encryptValueCmd = &cobra.Command{
	Use:   "encrypt-value VALUE",
	Short: "Encrypt a single value and print its placeholder key",
	Long: `Encrypt a single value and store it in the vault. Returns the placeholder
that you can manually paste into your YAML file.

Examples:

  gitops-vault encrypt-value --key db_password "mysecret123"
  echo "mysecret123" | xargs gitops-vault encrypt-value --key db_password`,
	Args: cobra.ExactArgs(1),
	RunE: runEncryptValue,
}

func runEncryptValue(cmd *cobra.Command, args []string) error {
	value := args[0]

	cfg, _ := config.Load()

	pubKeySource := evPublicKey
	if !cmd.Flags().Changed("public-key") && cfg.PublicKey != "" {
		pubKeySource = cfg.PublicKey
	}
	pubKey, err := vault.LoadPublicKey(pubKeySource)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}

	secretDir := evSecretDir
	if !cmd.Flags().Changed("secret-dir") && cfg.SecretDir != "" {
		secretDir = cfg.SecretDir
	}
	if secretDir == "" {
		secretDir = ".vault"
	}

	ph := placeholder.Generate(evKeyName)
	encrypted, err := vault.Encrypt(value, pubKey)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	store := vault.NewStore(secretDir)
	if err := store.Put(ph, encrypted); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Stored in %s/%s\n", secretDir, ph)
	fmt.Println(ph)
	return nil
}
