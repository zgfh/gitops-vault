package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".gitops-vault.yml"

// Config holds all persistent settings.
type Config struct {
	PublicKey     string   `yaml:"public_key"`
	PrivateKey    string   `yaml:"private_key,omitempty"`
	SecretDir     string   `yaml:"secret_dir"`
	SensitiveKeys []string `yaml:"sensitive_keys"`
	Exclude       []string `yaml:"exclude"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		SecretDir: ".vault",
	}
}

// FindPath locates the config file by walking up from current directory.
// Returns empty string if not found.
func FindPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// Load finds and loads the config file. Returns the default config if none exists.
func Load() (*Config, error) {
	path, err := FindPath()
	if err != nil {
		return nil, fmt.Errorf("find config: %w", err)
	}
	if path == "" {
		return Default(), nil
	}
	return LoadPath(path)
}

// LoadPath reads a config from a specific path.
func LoadPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config to the given path.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
