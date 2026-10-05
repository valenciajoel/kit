package bundle

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// RedactedPlaceholder replaces any secret value captured in a bundle.
const RedactedPlaceholder = "${REDACTED}"

var sensitiveExact = map[string]bool{
	"apikey": true, "api_key": true, "api-key": true, "api_keys": true,
	"token": true, "access_token": true, "refresh_token": true, "session_token": true,
	"authorization": true, "auth": true, "bearer": true,
	"cookie": true, "set-cookie": true,
	"password": true, "passwd": true, "secret": true,
	"client_secret": true, "secret_key": true, "private_key": true,
	"access_key": true, "private_key_id": true,
}

// sensitiveSubstr are specific compound tokens safe to match anywhere, since
// they are unlikely to appear inside a non-credential identifier.
var sensitiveSubstr = []string{
	"api_key", "apikey", "api-key",
	"access_key", "secret_key", "private_key", "client_secret",
	"authorization", "authentication",
}

// sensitiveSuffixes are matched only at the end of a key. Suffixes are kept
// deliberately narrow: matching a bare "_key"/"-key" produced false positives
// on identifiers like "topic_key" and the npm package "path-key".
var sensitiveSuffixes = []string{
	"_token", "_secret", "_password", "_passwd",
	"-token", "-secret", "-password", "-passwd",
}

// IsSensitiveKey reports whether a config key likely holds a credential.
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.Trim(k, `"'`)
	if k == "" {
		return false
	}
	if sensitiveExact[k] {
		return true
	}
	for _, sub := range sensitiveSubstr {
		if strings.Contains(k, sub) {
			return true
		}
	}
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}

// credentialValueRe matches well-known credential shapes even when no key name
// marks them, so an unnamed token in a comment is still caught.
var credentialValueRe = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{16,}|ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|ghs_[A-Za-z0-9]{20,}|ghu_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,}|AIza[0-9A-Za-z_-]{35,})\b`)

var keyValueRe = regexp.MustCompile(`^(\s*(?:export\s+|set\s+)?["']?([A-Za-z0-9_.\-]+)["']?\s*[:=]\s*)(.*)$`)

// Sanitize returns a copy of data with credential values redacted. JSON is
// handled structurally; everything else is scanned line by line. archivePath
// labels the redactions in the report.
func Sanitize(data []byte, archivePath string) ([]byte, []Redaction) {
	if out, reds, ok := sanitizeJSON(data, archivePath); ok {
		return out, reds
	}
	return sanitizeText(data, archivePath)
}

func sanitizeJSON(data []byte, archivePath string) ([]byte, []Redaction, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, nil, false
	}
	var root any
	if err := json.Unmarshal(trimmed, &root); err != nil {
		return nil, nil, false
	}
	var reds []Redaction
	root = redactJSON(root, archivePath, "", &reds)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, nil, false
	}
	out = append(out, '\n')
	return out, reds, true
}

func redactJSON(v any, archivePath, keyPath string, reds *[]Redaction) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			kp := k
			if keyPath != "" {
				kp = keyPath + "." + k
			}
			if IsSensitiveKey(k) {
				t[k] = RedactedPlaceholder
				*reds = append(*reds, Redaction{File: archivePath, Key: kp})
			} else {
				t[k] = redactJSON(val, archivePath, kp, reds)
			}
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactJSON(t[i], archivePath, keyPath, reds)
		}
		return t
	default:
		return v
	}
}

func sanitizeText(data []byte, archivePath string) ([]byte, []Redaction) {
	var reds []Redaction
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if m := keyValueRe.FindStringSubmatch(line); m != nil && IsSensitiveKey(m[2]) {
			reds = append(reds, Redaction{File: archivePath, Key: m[2], Line: i + 1})
			lines[i] = m[1] + `"` + RedactedPlaceholder + `"`
			continue
		}
		if credentialValueRe.MatchString(line) {
			reds = append(reds, Redaction{File: archivePath, Key: "(credential-value)", Line: i + 1})
			lines[i] = credentialValueRe.ReplaceAllString(line, RedactedPlaceholder)
		}
	}
	return []byte(strings.Join(lines, "\n")), reds
}
