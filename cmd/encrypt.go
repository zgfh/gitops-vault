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
	encryptPublicKey     string
	encryptSecretDir     string
	encryptDryRun        bool
	encryptSensitiveKeys []string
)

func init() {
	encryptCmd.Flags().StringVarP(&encryptPublicKey, "public-key", "k", "", "age public key (or set VAULT_PUBLIC_KEY env)")
	encryptCmd.Flags().StringVarP(&encryptSecretDir, "secret-dir", "d", ".vault", "directory to store encrypted secrets")
	encryptCmd.Flags().BoolVar(&encryptDryRun, "dry-run", false, "show what would be done without writing")
	encryptCmd.Flags().StringSliceVarP(&encryptSensitiveKeys, "sensitive-key", "s", nil, "additional sensitive key patterns")
	rootCmd.AddCommand(encryptCmd)
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file|dir...]",
	Short: "Encrypt sensitive values and replace with placeholders",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runEncrypt,
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	pubKey, err := vault.LoadPublicKey(encryptPublicKey)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}

	files, err := scanner.WalkYAML(args)
	if err != nil {
		return fmt.Errorf("walk paths: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No YAML files found.")
		return nil
	}

	store := vault.NewStore(encryptSecretDir)
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

		count := processEncrypt(&doc, pubKey, store)
		if count > 0 {
			fmt.Printf("%s: %d value(s) encrypted\n", file, count)

			if !encryptDryRun {
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
		}
		total += count
	}

	if total == 0 {
		fmt.Println("No sensitive values found.")
	} else {
		fmt.Printf("\nTotal: %d value(s) encrypted into %s/\n", total, encryptSecretDir)
	}
	return nil
}

// processEncrypt walks the YAML tree and replaces sensitive values with placeholders.
// Returns the number of replacements made.
func processEncrypt(doc *yaml.Node, pubKey string, store *vault.Store) int {
	count := 0

	yamledit.Walk(doc, func(node *yaml.Node, path []string, value string) *scanner.Finding {
		keyName := path[len(path)-1]

		// 1. YAML key-value: key name contains sensitive keyword
		if scanner.KeyContainsSensitive(keyName, encryptSensitiveKeys) {
			ph := placeholder.Generate(yamledit.KeyFromPath(path))
			encrypted, err := vault.Encrypt(value, pubKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warn: encrypt %s: %v\n", keyName, err)
				return nil
			}
			if !encryptDryRun {
				_ = store.Put(ph, encrypted)
			}
			fmt.Printf("  %s = *** -> %s\n", strings.Join(path, "."), ph)
			node.Value = ph
			count++
			return nil
		}

		// 2. Command args: --flag-name=VALUE
		if found, _ := scanner.HasSensitiveArg(value); found {
			result := value
			for _, re := range scanner.ArgSensitivePatterns() {
				for {
					loc := re.FindStringIndex(result)
					if loc == nil {
						break
					}
					full := result[loc[0]:loc[1]]
					parts := strings.SplitN(full, "=", 2)
					if len(parts) != 2 {
						break
					}
					flag, val := parts[0], parts[1]
					// Skip if already a placeholder
					if placeholder.IsPlaceholder(val) {
						break
					}
					keyHint := strings.TrimPrefix(flag, "--")
					keyHint = strings.ReplaceAll(keyHint, "-", "_")
					ph := placeholder.Generate(keyHint)

					encrypted, err := vault.Encrypt(val, pubKey)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  warn: encrypt arg %s: %v\n", keyHint, err)
						break
					}
					if !encryptDryRun {
						_ = store.Put(ph, encrypted)
					}
					fmt.Printf("  [ARG] %s=*** -> %s=%s\n", flag, flag, ph)
					result = result[:loc[0]] + flag + "=" + ph + result[loc[1]:]
					count++
				}
			}
			node.Value = result
			return nil
		}

		// 3. Embedded config content (toml/ini/properties in multi-line strings)
		if (strings.Contains(value, "\n") || strings.Contains(value, " = ") || strings.Contains(value, "=")) &&
			!strings.HasPrefix(value, "--") && len(value) > 10 {

			result := value
			matches := scanner.EmbeddedKeyValueRE.FindAllStringSubmatchIndex(result, -1)
			changed := false
			for i := len(matches) - 1; i >= 0; i-- {
				m := matches[i]
				if len(m) < 6 {
					continue
				}
				ekey := result[m[2]:m[3]]
				eval := result[m[4]:m[5]]
				if scanner.KeyContainsSensitive(ekey, encryptSensitiveKeys) {
					ph := placeholder.Generate(ekey)
					encrypted, err := vault.Encrypt(eval, pubKey)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  warn: encrypt embedded %s: %v\n", ekey, err)
						continue
					}
					if !encryptDryRun {
						_ = store.Put(ph, encrypted)
					}
					fmt.Printf("  [EMBEDDED] %s = *** -> %s = %s\n", ekey, ekey, ph)
					result = result[:m[4]] + ph + result[m[5]:]
					changed = true
					count++
				}
			}
			if changed {
				node.Value = result
			}
		}

		return nil
	})

	return count
}
