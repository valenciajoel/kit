package bundle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeJSONRedactsSensitiveKeys(t *testing.T) {
	in := []byte(`{"provider":{"apiKey":"sk-live-123","model":"gpt"},"safe":"value"}`)
	out, reds := Sanitize(in, "configs/opencode/opencode.json")

	if len(reds) == 0 {
		t.Fatal("expected at least one redaction")
	}
	if strings.Contains(string(out), "sk-live-123") {
		t.Fatal("secret leaked through JSON sanitization")
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("sanitized output is not valid JSON: %v", err)
	}
	if m["safe"] != "value" {
		t.Fatalf("non-sensitive value changed: %v", m["safe"])
	}
}

func TestSanitizeTextEnvAssignment(t *testing.T) {
	in := []byte("export GITHUB_TOKEN=ghp_abcdefghijklmnopqrstuv\nPATH=/usr/bin\n")
	out, reds := Sanitize(in, ".bashrc")

	if len(reds) == 0 {
		t.Fatal("expected a redaction")
	}
	if strings.Contains(string(out), "ghp_abcdefghijklmnopqrstuv") {
		t.Fatal("token leaked through text sanitization")
	}
	if !strings.Contains(string(out), "PATH=/usr/bin") {
		t.Fatal("non-sensitive line was modified")
	}
}

func TestSanitizeTextRawCredential(t *testing.T) {
	in := []byte("note = \"key is sk-abcdefghijklmnop1234\"\n")
	out, reds := Sanitize(in, "configs/foo")

	if len(reds) == 0 {
		t.Fatal("expected a redaction for an inline credential value")
	}
	if strings.Contains(string(out), "sk-abcdefghijklmnop1234") {
		t.Fatal("raw credential leaked")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"api_key", "API-KEY", "authorization", "My_TOKEN", "client_secret",
		"password", "openai_apiKey", "CONTEXT7_API_KEY", "GITHUB_TOKEN",
		"AWS_ACCESS_KEY_ID", "ANTHROPIC_API_KEY",
	}
	for _, k := range sensitive {
		if !IsSensitiveKey(k) {
			t.Errorf("expected %q to be sensitive", k)
		}
	}
	safe := []string{
		"model", "theme", "font_size", "keybind", "keybinds", "timeout", "provider",
		"topic_key", "path-key", "max_tokens", "signature", "monkey",
	}
	for _, k := range safe {
		if IsSensitiveKey(k) {
			t.Errorf("expected %q to be safe", k)
		}
	}
}
