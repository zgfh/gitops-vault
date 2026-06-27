package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/zzg/gitops-vault/pkg/config"
	"github.com/zzg/gitops-vault/pkg/scanner"
	"github.com/zzg/gitops-vault/pkg/yamledit"
)

var scanSensitiveKeys []string

func init() {
	scanCmd.Flags().StringSliceVarP(&scanSensitiveKeys, "sensitive-key", "s", nil, "override sensitive key patterns (default from config)")
	rootCmd.AddCommand(scanCmd)
}

var scanCmd = &cobra.Command{
	Use:   "scan [file|dir...]",
	Short: "Scan for sensitive values without modifying files",
	Long: `Scan YAML files and report sensitive values that would be encrypted.
This is a dry-run that does not modify any files.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runScan,
}

func runScan(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()

	sensitiveKeys := scanSensitiveKeys
	if !cmd.Flags().Changed("sensitive-key") && len(cfg.SensitiveKeys) > 0 {
		sensitiveKeys = cfg.SensitiveKeys
	}

	files, err := scanner.WalkYAML(args, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("walk paths: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No YAML files found.")
		return nil
	}

	total := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
			continue
		}

		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", file, err)
			continue
		}

		findings := yamledit.Walk(&doc, func(node *yaml.Node, path []string, value string) *scanner.Finding {
			keyName := path[len(path)-1]

			if scanner.KeyContainsSensitive(keyName, sensitiveKeys) {
				return &scanner.Finding{
					YAMLPath:   strings.Join(path, "."),
					Value:      value,
					KeyHint:    yamledit.KeyFromPath(path),
					LineNumber: node.Line,
				}
			}

			if found, _ := scanner.HasSensitiveArg(value); found {
				return &scanner.Finding{
					YAMLPath:   strings.Join(path, "."),
					Value:      value,
					KeyHint:    yamledit.KeyFromPath(path),
					LineNumber: node.Line,
					IsArg:      true,
				}
			}

			if (strings.Contains(value, "\n") || strings.Contains(value, " = ")) &&
				!strings.HasPrefix(value, "--") && len(value) > 10 {
				match := scanner.EmbeddedKeyValueRE.FindAllStringSubmatch(value, -1)
				for _, m := range match {
					if len(m) >= 3 && scanner.KeyContainsSensitive(m[1], sensitiveKeys) {
						return &scanner.Finding{
							YAMLPath:   strings.Join(path, "."),
							Value:      fmt.Sprintf("%s = %s", m[1], m[2]),
							KeyHint:    m[1],
							LineNumber: node.Line,
							IsEmbedded: true,
						}
					}
				}
			}

			return nil
		})

		if len(findings) > 0 {
			fmt.Printf("%s:\n", file)
			for _, f := range findings {
				tag := ""
				if f.IsArg {
					tag = "[ARG] "
				} else if f.IsEmbedded {
					tag = "[EMBEDDED] "
				}
				display := f.Value
				if len(display) > 40 {
					display = display[:40] + "..."
				}
				fmt.Printf("  %s%s = %s\n", tag, f.YAMLPath, display)
			}
			total += len(findings)
		}
	}

	if total == 0 {
		fmt.Println("No sensitive values found.")
	} else {
		fmt.Printf("\nTotal: %d sensitive value(s) found\n", total)
	}
	return nil
}
