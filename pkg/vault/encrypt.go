package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
)

// Encrypt encrypts a plaintext value with the given age public key.
// Returns the base64-encoded encrypted blob.
func Encrypt(plaintext string, publicKey string) (string, error) {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return "", fmt.Errorf("create encrypt writer: %w", err)
	}
	if _, err := io.WriteString(w, plaintext); err != nil {
		return "", fmt.Errorf("write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close encrypt writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Decrypt decrypts a base64-encoded age-encrypted blob with the given private key.
func Decrypt(encoded string, privateKey string) (string, error) {
	encrypted, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	identity, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	reader := bytes.NewReader(encrypted)
	r, err := age.Decrypt(reader, identity)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", fmt.Errorf("read decrypted: %w", err)
	}

	return buf.String(), nil
}

// LoadPublicKey reads the age public key from common locations.
func LoadPublicKey(flagPath string) (string, error) {
	if flagPath != "" {
		// Direct key value
		if key := extractKey(flagPath); key != flagPath || strings.HasPrefix(key, "age1") {
			return key, nil
		}
		return readKeyFile(flagPath)
	}
	if s := os.Getenv("VAULT_PUBLIC_KEY"); s != "" {
		return s, nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		if data, err := os.ReadFile(home + "/.age/vault.pub"); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", fmt.Errorf("no public key found: set --public-key, VAULT_PUBLIC_KEY, or create ~/.age/vault.pub")
}

// LoadPrivateKey reads the age private key from common locations.
func LoadPrivateKey(flagPath string) (string, error) {
	if flagPath != "" {
		// Direct key value (or multi-line content containing a key)
		if key := extractKey(flagPath); key != flagPath || strings.HasPrefix(key, "AGE-SECRET-KEY-1") {
			return key, nil
		}
		return readKeyFile(flagPath)
	}
	if s := os.Getenv("AGE_KEY"); s != "" {
		return s, nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		if data, err := os.ReadFile(home + "/.age/key.txt"); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", fmt.Errorf("no private key found: set --private-key, AGE_KEY, or create ~/.age/key.txt")
}

func readKeyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return extractKey(string(data)), nil
}

// extractKey scans multi-line content and extracts the age key (age1... or AGE-SECRET-KEY-1...).
func extractKey(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "age1") || strings.HasPrefix(line, "AGE-SECRET-KEY-1") {
			return line
		}
	}
	return strings.TrimSpace(content)
}
