package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

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

func TestListAndCurrent(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "omakub", "themes")
	if err := os.MkdirAll(filepath.Join(dir, "nord"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tokyo-night"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "not-a-theme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nameFile := filepath.Join(home, ".config", "omakub", "current", "theme.name")
	if err := os.MkdirAll(filepath.Dir(nameFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nameFile, []byte("nord\n"), 0o644); err != nil {
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
	if len(themes) != 2 {
		t.Fatalf("themes = %v, want 2", themes)
	}
	if themes[0].Slug != "nord" || themes[1].Slug != "tokyo-night" {
		t.Fatalf("order = %v (want nord, tokyo-night)", themes)
	}
	if cur := Current(cfg, env, target.Linux); cur != "nord" {
		t.Fatalf("current = %q, want nord", cur)
	}
}

func TestListMissingDirIsEmpty(t *testing.T) {
	home := t.TempDir()
	cfg := &manifest.Themes{Dir: "~/.config/omakub/themes", SetCommand: "x"}
	env := inventory.Environment{OS: inventory.Linux, Home: home, ConfigDir: filepath.Join(home, ".config")}

	themes, err := List(cfg, env, target.Linux)
	if err != nil || themes != nil {
		t.Fatalf("themes = %v, err = %v", themes, err)
	}
}
