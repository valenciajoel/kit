package restore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valenciajoel/kit/internal/bundle"
	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

func zellijManifest() *manifest.Manifest {
	return &manifest.Manifest{Version: 1, Components: []manifest.Component{{
		ID:   "zellij",
		Name: "Zellij",
		Kind: "terminal-multiplexer",
		Configs: []manifest.ConfigSpec{{
			Src:  "~/.config/zellij/config.kdl",
			Dest: map[string]string{"linux": "~/.config/zellij/config.kdl", "windows": "%APPDATA%/zellij/config.kdl"},
		}},
		Install: map[string]manifest.Install{"linux": {Method: "apt", Pkg: "zellij"}},
	}}}
}

func TestRestoreRendersToNewHome(t *testing.T) {
	srcHome := filepath.Join(t.TempDir(), "home")
	mustWrite(t, filepath.Join(srcHome, ".config", "zellij", "config.kdl"),
		"theme_dir \""+srcHome+"/.config/zellij/themes\"\n")

	m := zellijManifest()
	envA := inventory.Environment{OS: inventory.Linux, Home: srcHome}
	exp, err := bundle.Export(bundle.Options{
		Manifest: m,
		Env:      envA,
		Target:   target.Linux,
		OutDir:   filepath.Join(t.TempDir(), "out"),
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstHome := filepath.Join(t.TempDir(), "home")
	envB := inventory.Environment{OS: inventory.Linux, Home: dstHome}
	res, err := Restore(Options{BundlePath: exp.BundlePath, Manifest: m, Env: envB, Target: target.Linux})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	want := filepath.Join(dstHome, ".config", "zellij", "config.kdl")
	if len(res.Written) != 1 || res.Written[0] != want {
		t.Fatalf("written = %v, want [%s]", res.Written, want)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if !strings.Contains(string(got), dstHome) {
		t.Fatalf("home not rendered for new machine: %s", got)
	}
	if strings.Contains(string(got), srcHome) {
		t.Fatalf("source home leaked into restored config: %s", got)
	}
}

func TestRestoreWindowsDestination(t *testing.T) {
	srcHome := filepath.Join(t.TempDir(), "home")
	mustWrite(t, filepath.Join(srcHome, ".config", "zellij", "config.kdl"), "theme \"default\"\n")

	appdata := filepath.Join(t.TempDir(), "AppData", "Roaming")
	t.Setenv("APPDATA", appdata)

	m := zellijManifest()
	exp, err := bundle.Export(bundle.Options{
		Manifest: m,
		Env:      inventory.Environment{OS: inventory.Linux, Home: srcHome},
		Target:   target.Linux,
		OutDir:   filepath.Join(t.TempDir(), "out"),
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	res, err := Restore(Options{
		BundlePath: exp.BundlePath,
		Manifest:   m,
		Env:        inventory.Environment{OS: inventory.Windows, Home: filepath.Join(t.TempDir(), "user")},
		Target:     target.Windows,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	want := filepath.Join(appdata, "zellij", "config.kdl")
	if len(res.Written) != 1 || res.Written[0] != want {
		t.Fatalf("windows dest = %v, want [%s]", res.Written, want)
	}
}

func TestSeedsWritesBaseline(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	m := &manifest.Manifest{Version: 1, Components: []manifest.Component{{
		ID:   "starship",
		Name: "Starship",
		Kind: "shell-prompt",
		Configs: []manifest.ConfigSpec{{
			Src:  "~/.config/starship.toml",
			Dest: map[string]string{"linux": "~/.config/starship.toml"},
		}},
		Seed: "configs/starship.toml",
	}}}

	res, err := Seeds(Options{
		Manifest: m,
		Env:      inventory.Environment{OS: inventory.Linux, Home: home},
		Target:   target.Linux,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("seeds: %v", err)
	}
	want := filepath.Join(home, ".config", "starship.toml") + " (seed)"
	if len(res.Written) != 1 || res.Written[0] != want {
		t.Fatalf("written = %v, want [%s]", res.Written, want)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
