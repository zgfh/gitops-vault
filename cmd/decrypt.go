package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/zzg/gitops-vault/pkg/placeholder"
	"github.com/zzg/gitops-vault/pkg/scanner"
	"github.com/zzg/gitops-vault/pkg/vault"
	"github.com/zzg/gitops-vault/pkg/yamledit"
)

var (
	decryptPrivateKey string
	decryptSecretDir  string
)

func init() {
	decryptCmd.Flags().StringVarP(&decryptPrivateKey, "private-key", "k", "", "age private key (or set AGE_KEY env)")
	decryptCmd.Flags().StringVarP(&decryptSecretDir, "secret-dir", "d", ".vault", "directory containing encrypted secrets")
	rootCmd.AddCommand(decryptCmd)
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt [file|dir...]",
	Short: "Decrypt and restore placeholders to original values",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDecrypt,
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	privKey, err := vault.LoadPrivateKey(decryptPrivateKey)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	files, err := scanner.WalkYAML(args)
	if err != nil {
		return fmt.Errorf("walk paths: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No YAML files found.")
		return nil
	}

	store := vault.NewStore(decryptSecretDir)
	total := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}

		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", file, err)
		}

		count := processDecrypt(&doc, privKey, store)
		if count > 0 {
			fmt.Printf("%s: %d value(s) decrypted\n", file, count)

			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&doc); err != nil {
				return fmt.Errorf("marshal %s: %w", file, err)
			}
			enc.Close()
			if err := os.WriteFile(file, buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("write %s: %w", file, err)
			}
		}
		total += count
	}

	if total == 0 {
		fmt.Println("No placeholders found.")
	} else {
		fmt.Printf("\nTotal: %d value(s) decrypted from %s/\n", total, decryptSecretDir)
	}
	return nil
}

func processDecrypt(doc *yaml.Node, privKey string, store *vault.Store) int {
	count := 0
	// Cache decrypted values: ph -> original value
	cache := make(map[string]string)

	yamledit.Walk(doc, func(node *yaml.Node, path []string, value string) *scanner.Finding {
		if !placeholder.IsPlaceholder(value) {
			// Multi-value: check for placeholders within string
			phs := placeholder.FindAll(value)
			if len(phs) == 0 {
				return nil
			}
			result := value
			for _, ph := range phs {
				original, err := resolveDecrypted(ph, privKey, store, cache)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  warn: %s: %v\n", ph, err)
					continue
				}
				result = strings.ReplaceAll(result, ph, original)
				count++
			}
			node.Value = result
			return nil
		}

		// Single placeholder as the whole value
		ph := value
		original, err := resolveDecrypted(ph, privKey, store, cache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warn: %s: %v\n", ph, err)
			return nil
		}
		fmt.Printf("  %s = %s -> ***\n", strings.Join(path, "."), ph)
		node.Value = original
		count++
		return nil
	})

	return count
}

func resolveDecrypted(ph, privKey string, store *vault.Store, cache map[string]string) (string, error) {
	if val, ok := cache[ph]; ok {
		return val, nil
	}
	encrypted, err := store.Get(ph)
	if err != nil {
		return "", err
	}
	original, err := vault.Decrypt(encrypted, privKey)
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", ph, err)
	}
	cache[ph] = original
	return original, nil
}
