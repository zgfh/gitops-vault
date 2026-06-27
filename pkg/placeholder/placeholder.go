package placeholder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const Prefix = "VAULT"

// placeholderPattern matches VAULT_{KEY}_{TIMESTAMP} (anywhere in text)
var placeholderPattern = regexp.MustCompile(`VAULT_[A-Z][A-Z0-9_]*_\d{10,}`)

// exactPlaceholderPattern matches a string that is exactly a placeholder
var exactPlaceholderPattern = regexp.MustCompile(`^VAULT_[A-Z][A-Z0-9_]*_\d{10,}$`)

var counter int64

// Generate creates a placeholder from a key name. The key is normalized to
// uppercase with non-alphanumeric chars replaced by underscores.
// Uses UnixNano + atomic counter for uniqueness.
func Generate(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ToUpper(key)

	var b strings.Builder
	for i, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == '_' {
			b.WriteRune('_')
		} else {
			if i > 0 && key[i-1] != '_' {
				b.WriteRune('_')
			}
		}
	}
	normalized := strings.Trim(b.String(), "_")

	// Collapse consecutive underscores
	for strings.Contains(normalized, "__") {
		normalized = strings.ReplaceAll(normalized, "__", "_")
	}

	if normalized == "" {
		normalized = "SECRET"
	}

	counter++
	ts := time.Now().UnixNano()
	return fmt.Sprintf("VAULT_%s_%d", normalized, ts+int64(counter))
}

// IsPlaceholder checks if a string is exactly a vault placeholder (whole-value match).
func IsPlaceholder(s string) bool {
	return exactPlaceholderPattern.MatchString(s)
}

// ContainsPlaceholder checks if a string contains any vault placeholder.
func ContainsPlaceholder(s string) bool {
	return placeholderPattern.MatchString(s)
}

// parsePattern is used for extracting key and timestamp from a placeholder.
var parsePattern = regexp.MustCompile(`^VAULT_([A-Z][A-Z0-9_]*)_(\d{10,})$`)

// Parse extracts the key and timestamp from a placeholder string.
func Parse(s string) (key string, timestamp int64, err error) {
	matches := parsePattern.FindStringSubmatch(s)
	if len(matches) != 3 {
		return "", 0, fmt.Errorf("not a valid placeholder: %s", s)
	}
	ts, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return matches[1], ts, nil
}

// FindAll finds all placeholder strings in text.
func FindAll(text string) []string {
	return placeholderPattern.FindAllString(text, -1)
}
