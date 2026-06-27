package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultSecretDir = ".vault"

// Store manages reading and writing encrypted secrets to the .vault directory.
type Store struct {
	Dir string
}

// NewStore creates a Store with the given directory. If dir is empty, defaults to ".vault".
func NewStore(dir string) *Store {
	if dir == "" {
		dir = DefaultSecretDir
	}
	return &Store{Dir: dir}
}

// Put writes an encrypted value to the vault directory.
func (s *Store) Put(placeholder, encrypted string) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return fmt.Errorf("create vault dir: %w", err)
	}
	path := filepath.Join(s.Dir, placeholder)
	return os.WriteFile(path, []byte(encrypted+"\n"), 0600)
}

// Get reads and returns the encrypted value for a placeholder.
func (s *Store) Get(placeholder string) (string, error) {
	path := filepath.Join(s.Dir, placeholder)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("vault entry not found: %s", placeholder)
		}
		return "", err
	}
	// Trim trailing newline
	enc := string(data)
	if len(enc) > 0 && enc[len(enc)-1] == '\n' {
		enc = enc[:len(enc)-1]
	}
	return enc, nil
}

// All returns all placeholder names stored in the vault.
func (s *Store) All() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
