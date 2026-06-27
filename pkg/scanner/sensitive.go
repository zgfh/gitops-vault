package scanner

import "regexp"

// DefaultSensitiveKeys are patterns considered sensitive when found in key names.
var DefaultSensitiveKeys = []string{
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
}

// KeyContainsSensitive checks if a key name contains any sensitive pattern.
func KeyContainsSensitive(key string, extraPatterns []string) bool {
	all := append(DefaultSensitiveKeys, extraPatterns...)
	for _, p := range all {
		q := regexp.QuoteMeta(p)
		// Use (?:^|_|\\W) as left boundary so patterns match inside underscored keys like api_token
		if matches, _ := regexp.MatchString("(?i)(?:^|_|\\W)"+q+"(?:$|_|\\W)", key); matches {
			return true
		}
	}
	return false
}

// sensitiveSuffixes are the suffixes for sensitive command args.
var sensitiveSuffixes = []string{"password", "secret", "token", "key", "apikey", "api-key"}

// ArgSensitivePatterns returns regex patterns for detecting sensitive values in
// command-line arguments like "--db-password=mysecret".
func ArgSensitivePatterns() []*regexp.Regexp {
	var res []*regexp.Regexp
	for _, suffix := range sensitiveSuffixes {
		re := regexp.MustCompile(`--\S*` + regexp.QuoteMeta(suffix) + `\s*=\s*\S+`)
		res = append(res, re)
	}
	return res
}

// EmbeddedKeyValueRE matches key=value patterns in embedded config content (properties, ini, toml).
var EmbeddedKeyValueRE = regexp.MustCompile(`(?m)^\s*(\S+)\s*[=:]\s*["']?([^"'\n\r]+)["']?\s*$`)

// HasSensitiveArg checks if a string contains a sensitive command-line argument
// and returns the matched positions.
func HasSensitiveArg(s string) (bool, []Match) {
	var matches []Match
	for _, re := range ArgSensitivePatterns() {
		locs := re.FindAllStringIndex(s, -1)
		for _, loc := range locs {
			matches = append(matches, Match{Start: loc[0], End: loc[1], Full: s[loc[0]:loc[1]]})
		}
	}
	return len(matches) > 0, matches
}

// Match represents a location of a sensitive value in a string.
type Match struct {
	Start int
	End   int
	Full  string
}
