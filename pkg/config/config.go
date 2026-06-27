package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".gitops-vault.yml"

// Rule defines a per-file matching rule. First match wins.
// Only non-zero fields override the global defaults.
type Rule struct {
	PathRegex     string   `yaml:"path_regex"`
	PublicKey     string   `yaml:"public_key,omitempty"`
	SecretDir     string   `yaml:"secret_dir,omitempty"`
	SensitiveKeys []string `yaml:"sensitive_keys,omitempty"`
	Exclude       []string `yaml:"exclude,omitempty"`

	compiled *regexp.Regexp `yaml:"-"`
}

// Config holds all persistent settings.
type Config struct {
	PublicKey     string   `yaml:"public_key"`
	PrivateKey    string   `yaml:"private_key,omitempty"`
	SecretDir     string   `yaml:"secret_dir"`
	SensitiveKeys []string `yaml:"sensitive_keys"`
	Exclude       []string `yaml:"exclude"`
	Rules         []Rule   `yaml:"rules,omitempty"`
}

// Default returns a Config with sensible defaults including built-in sensitive key patterns.
func Default() *Config {
	return &Config{
		SecretDir: ".vault",
		SensitiveKeys: []string{
			"password", "passwd", "pwd",
			"secret", "token",
			"api_key", "apikey", "api_secret", "apisecret",
			"private_key", "privatekey",
			"access_key", "accesskey",
			"secret_key", "secretkey",
			"db_password", "db_passwd",
			"auth_token", "bearer_token",
			"client_secret",
			"encryption_key", "signing_key",
			"credential",
		},
		Exclude: []string{
			".git/",
			".vault/",
			"vendor/",
			"node_modules/",
		},
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
	if err := cfg.Compile(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Compile pre-compiles all rule regexes. Call after loading/unmarshaling.
func (c *Config) Compile() error {
	for i := range c.Rules {
		re, err := regexp.Compile(c.Rules[i].PathRegex)
		if err != nil {
			return fmt.Errorf("rule %d path_regex %q: %w", i, c.Rules[i].PathRegex, err)
		}
		c.Rules[i].compiled = re
	}
	return nil
}

// FileConfig returns the effective configuration for a specific file path
// by merging the first matching rule on top of global defaults.
func (c *Config) FileConfig(filePath string) *Config {
	// Start with a copy of globals
	cfg := &Config{
		PublicKey:     c.PublicKey,
		SecretDir:     c.SecretDir,
		SensitiveKeys: append([]string(nil), c.SensitiveKeys...),
		Exclude:       append([]string(nil), c.Exclude...),
	}

	for _, r := range c.Rules {
		if r.compiled == nil {
			continue
		}
		if r.compiled.MatchString(filePath) {
			if r.PublicKey != "" {
				cfg.PublicKey = r.PublicKey
			}
			if r.SecretDir != "" {
				cfg.SecretDir = r.SecretDir
			}
			if len(r.SensitiveKeys) > 0 {
				cfg.SensitiveKeys = append([]string(nil), r.SensitiveKeys...)
			}
			if len(r.Exclude) > 0 {
				cfg.Exclude = append([]string(nil), r.Exclude...)
			}
			break // first match wins
		}
	}

	return cfg
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
