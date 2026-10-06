package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

func containsSlug(themes []Theme, slug string) bool {
	for _, th := range themes {
		if th.Slug == slug {
			return true
		}
	}
	return false
}

func TestLabel(t *testing.T) {
	cases := map[string]string{
		"tokyo-night": "Tokyo Night",
		"gruvbox":     "Gruvbox",
		"rose-pine":   "Rose Pine",
		"retro-82":    "Retro 82",
	}
	for in, want := range cases {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("Tokyo Night"); got != "tokyo-night" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestListMergesLocalAndBundled(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "omakub", "themes")
	if err := os.MkdirAll(filepath.Join(dir, "my-theme"), 0o755); err != nil {
		t.Fatal(err)
	}
	nameFile := filepath.Join(home, ".config", "omakub", "current", "theme.name")
	if err := os.MkdirAll(filepath.Dir(nameFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nameFile, []byte("my-theme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &manifest.Themes{
		Dir:        "~/.config/omakub/themes",
		NameFile:   "~/.config/omakub/current/theme.name",
		SetCommand: "x",
	}
	env := inventory.Environment{OS: inventory.Linux, Home: home, ConfigDir: filepath.Join(home, ".config")}

	themes, err := List(cfg, env, target.Linux)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSlug(themes, "my-theme") {
		t.Fatal("local theme missing from list")
	}
	if !containsSlug(themes, "nord") {
		t.Fatal("bundled theme missing from list")
	}
	if cur := Current(cfg, env, target.Linux); cur != "my-theme" {
		t.Fatalf("current = %q, want my-theme", cur)
	}
}

func TestListBundledWithoutLocalDir(t *testing.T) {
	home := t.TempDir()
	cfg := &manifest.Themes{Dir: "~/.config/omakub/themes", SetCommand: "x"}
	env := inventory.Environment{OS: inventory.Linux, Home: home, ConfigDir: filepath.Join(home, ".config")}

	themes, err := List(cfg, env, target.Linux)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSlug(themes, "tokyo-night") {
		t.Fatal("expected bundled themes even with no local directory")
	}
}

func TestRenderAlacritty(t *testing.T) {
	out, err := RenderAlacritty("tokyo-night")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[colors.normal]") {
		t.Fatal("rendered output missing [colors.normal]")
	}
	if strings.Contains(s, "{{") {
		t.Fatalf("unsubstituted placeholder remains:\n%s", s)
	}
	if !strings.Contains(s, "#") {
		t.Fatal("rendered output has no colors")
	}
}

func TestZellijFile(t *testing.T) {
	out, err := ZellijFile("nord")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "themes {") {
		t.Fatal("bundled file is not a zellij theme")
	}
}
