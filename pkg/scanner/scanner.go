package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// YAMLFileExtensions are the recognized YAML file suffixes.
var YAMLFileExtensions = []string{".yaml", ".yml"}

// FileResult holds scan results for a single file.
type FileResult struct {
	Path     string
	Findings []Finding
}

// Finding represents a single sensitive value discovered in a file.
type Finding struct {
	YAMLPath    string // e.g. "stringData.db_password"
	Value       string // the sensitive value found
	KeyHint     string // derived key name for placeholder generation
	LineNumber  int
	IsEmbedded  bool   // true if found inside a multi-line string value
	IsArg       bool   // true if found in a command arg
	ReplaceFrom int    // start offset in the string (for embedded content)
	ReplaceTo   int    // end offset in the string (for embedded content)
}

// WalkYAML walks the given paths and returns all YAML files found.
// excludePatterns are simple glob patterns for paths to skip (e.g., "vendor/").
func WalkYAML(paths []string, excludePatterns []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					base := filepath.Base(path)
					if strings.HasPrefix(base, ".") && base != "." {
						return filepath.SkipDir
					}
					for _, ex := range excludePatterns {
						if matched, _ := filepath.Match(ex, base); matched {
							return filepath.SkipDir
						}
					}
					return nil
				}
				for _, ext := range YAMLFileExtensions {
					if strings.HasSuffix(strings.ToLower(path), ext) {
						files = append(files, path)
						break
					}
				}
				return nil
			})
		} else {
			files = append(files, p)
		}
	}
	return files, nil
}
