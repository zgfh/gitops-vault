package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/zzg/gitops-vault/pkg/config"
	"github.com/zzg/gitops-vault/pkg/placeholder"
	"github.com/zzg/gitops-vault/pkg/scanner"
	"github.com/zzg/gitops-vault/pkg/vault"
	"github.com/zzg/gitops-vault/pkg/yamledit"
)

var (
	decryptPrivateKey string
	decryptSecretDir  string
	decryptWrite      bool
)

func init() {
	decryptCmd.Flags().StringVarP(&decryptPrivateKey, "private-key", "k", "", "age private key (or set AGE_KEY env)")
	decryptCmd.Flags().StringVarP(&decryptSecretDir, "secret-dir", "d", ".vault", "directory containing encrypted secrets")
	decryptCmd.Flags().BoolVarP(&decryptWrite, "write", "w", false, "modify files in place (default: output to stdout)")
	rootCmd.AddCommand(decryptCmd)
}

var decryptCmd = &cobra.Command{
	Use:     "decrypt [file|dir...]",
	Aliases: []string{"d"},
	Short:   "Decrypt and restore placeholders to original values",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDecrypt,
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()

	privKeySource := decryptPrivateKey
	if !cmd.Flags().Changed("private-key") && cfg.PrivateKey != "" {
		privKeySource = cfg.PrivateKey
	}
	privKey, err := vault.LoadPrivateKey(privKeySource)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	secretDir := decryptSecretDir
	if !cmd.Flags().Changed("secret-dir") && cfg.SecretDir != "" {
		secretDir = cfg.SecretDir
	}

	files, err := scanner.WalkYAML(args, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("walk paths: %w", err)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No YAML files found.")
		return nil
	}

	store := vault.NewStore(secretDir)
	total := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}

		// Decode all YAML documents (support multi-document files with ---)
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var docs []*yaml.Node
		for {
			var doc yaml.Node
			if err := decoder.Decode(&doc); err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("parse %s: %w", file, err)
			}
			docs = append(docs, &doc)
		}

		if len(docs) == 0 {
			continue
		}

		fileCount := 0
		for _, doc := range docs {
			fileCount += processDecrypt(doc, privKey, store)
		}
		count := fileCount
		if count > 0 {
			fmt.Fprintf(os.Stderr, "%s: %d value(s) decrypted\n", file, count)

			var buf bytes.Buffer
			for i, doc := range docs {
				out, err := yamledit.MarshalNode(doc)
				if err != nil {
					return fmt.Errorf("marshal %s: %w", file, err)
				}
				if i > 0 {
					buf.WriteString("---\n")
				}
				buf.Write(out)
			}
			if decryptWrite {
				if err := os.WriteFile(file, buf.Bytes(), 0644); err != nil {
					return fmt.Errorf("write %s: %w", file, err)
				}
			} else {
				os.Stdout.Write(buf.Bytes())
			}
		}
		total += count
	}

	if total == 0 {
		fmt.Fprintln(os.Stderr, "No placeholders found.")
	} else {
		fmt.Fprintf(os.Stderr, "\nTotal: %d value(s) decrypted from %s/\n", total, secretDir)
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
		fmt.Fprintf(os.Stderr, "  %s = %s -> ***\n", strings.Join(path, "."), ph)
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
