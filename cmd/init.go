package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
	"github.com/spf13/cobra"

	"github.com/zzg/gitops-vault/pkg/config"
)

var (
	initForce       bool
	initGenerateKey bool
	initKeyDir      string
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing config")
	initCmd.Flags().BoolVar(&initGenerateKey, "generate-key", false, "generate an age key pair")
	initCmd.Flags().StringVar(&initKeyDir, "key-dir", "", "directory for generated keys (default ~/.age)")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gitops-vault configuration",
	Long: `Create a .gitops-vault.yml configuration file with sensible defaults.

If --generate-key is set, an age key pair is also generated and the public
key is written into the config file.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath := filepath.Join(cwd, config.ConfigFileName)

	// Check existing
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("%s already exists (use --force to overwrite)", config.ConfigFileName)
	}

	cfg := config.Default()

	// Generate key pair if requested
	if initGenerateKey {
		pub, priv, err := generateAgeKey()
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}

		keyDir := initKeyDir
		if keyDir == "" {
			home, _ := os.UserHomeDir()
			keyDir = filepath.Join(home, ".age")
		}

		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("create key dir: %w", err)
		}

		pubFile := filepath.Join(keyDir, "vault.pub")
		privFile := filepath.Join(keyDir, "key.txt")

		if err := os.WriteFile(pubFile, []byte(pub+"\n"), 0644); err != nil {
			return fmt.Errorf("write public key: %w", err)
		}
		if err := os.WriteFile(privFile, []byte(priv+"\n"), 0600); err != nil {
			return fmt.Errorf("write private key: %w", err)
		}

		cfg.PublicKey = pub

		fmt.Printf("Public key:  %s\n", pubFile)
		fmt.Printf("Private key: %s\n", privFile)
	}

	if err := cfg.Save(configPath); err != nil {
		return err
	}

	fmt.Printf("Config written: %s\n", configPath)
	return nil
}

func generateAgeKey() (pub string, priv string, err error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", err
	}
	return identity.Recipient().String(), identity.String(), nil
}
