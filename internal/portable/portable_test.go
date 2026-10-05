package portable

import (
	"strings"
	"testing"

	"github.com/valenciajoel/kit/internal/inventory"
)

func TestNormalizeReplacesHome(t *testing.T) {
	env := inventory.Environment{Home: "/home/joel"}
	in := []byte("theme_dir \"/home/joel/.config/zellij/themes\"\n")
	out := Normalize(in, env)
	if strings.Contains(string(out), "/home/joel") {
		t.Fatalf("home path leaked: %s", out)
	}
	if !strings.Contains(string(out), "~/.config/zellij/themes") {
		t.Fatalf("expected ~ path, got: %s", out)
	}
}

func TestNormalizeWindowsEscapedHome(t *testing.T) {
	env := inventory.Environment{Home: `C:\Users\joel`}
	in := []byte(`{"config":"C:\\Users\\joel\\.config\\x"}`)
	out := Normalize(in, env)
	if strings.Contains(string(out), "joel") {
		t.Fatalf("windows home leaked: %s", out)
	}
}

func TestRenderRestoresHome(t *testing.T) {
	env := inventory.Environment{Home: "/home/joel"}
	in := []byte("shell = \"~\"/.zshrc\n")
	out := Render(in, env)
	if !strings.Contains(string(out), "/home/joel") {
		t.Fatalf("expected home restored, got: %s", out)
	}
	if strings.Contains(string(out), "~") {
		t.Fatalf("tilde left behind: %s", out)
	}
}

func TestRoundTrip(t *testing.T) {
	env := inventory.Environment{Home: "/home/joel"}
	original := "import = [\"/home/joel/.config/omakub/theme.toml\"]\n"
	normalized := Normalize([]byte(original), env)
	rendered := Render(normalized, env)
	if string(rendered) != original {
		t.Fatalf("round trip mismatch:\n got %q\nwant %q", rendered, original)
	}
}
