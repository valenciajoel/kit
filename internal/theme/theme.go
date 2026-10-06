// Package theme discovers the themes available on this machine and applies a
// chosen one by delegating to the owner's own command (for example Omakub's
// omakub-theme-set), so kit never reimplements theme rendering.
package theme

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/state"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/kits"
)

// Theme is one selectable theme.
type Theme struct {
	Slug  string
	Label string
}

// List returns the themes available on this machine: the ones bundled in kit
// (so theming works on any OS) merged with any found in the configured
// directory (for example Omakub's).
func List(cfg *manifest.Themes, env inventory.Environment, tgt target.Target) ([]Theme, error) {
	bySlug := make(map[string]Theme)

	if entries, err := fs.ReadDir(kits.FS, "themes"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				bySlug[e.Name()] = Theme{Slug: e.Name(), Label: Label(e.Name())}
			}
		}
	}

	if cfg != nil && cfg.Dir != "" {
		dir := target.ExpandPath(tgt, env, cfg.Dir)
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
				continue
			}
			bySlug[e.Name()] = Theme{Slug: e.Name(), Label: Label(e.Name())}
		}
	}

	out := make([]Theme, 0, len(bySlug))
	for _, t := range bySlug {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out, nil
}

// ZellijFile returns the bundled Zellij theme KDL for a slug.
func ZellijFile(slug string) ([]byte, error) {
	return kits.FS.ReadFile("themes/" + slug + "/zellij.kdl")
}

// RenderAlacritty renders the Alacritty theme for a slug from its palette and
// the shared template, mirroring Omakub's template substitution.
func RenderAlacritty(slug string) ([]byte, error) {
	colors, err := kits.FS.ReadFile("themes/" + slug + "/colors.toml")
	if err != nil {
		return nil, err
	}
	tpl, err := kits.FS.ReadFile("themes/alacritty.toml.tpl")
	if err != nil {
		return nil, err
	}
	out := tpl
	for key, val := range parseColors(colors) {
		out = bytes.ReplaceAll(out, []byte("{{ "+key+" }}"), []byte(val))
		out = bytes.ReplaceAll(out, []byte("{{"+key+"}}"), []byte(val))
	}
	return out, nil
}

// parseColors reads a simple `key = "value"` palette file.
func parseColors(data []byte) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		if i := strings.Index(val, "#"); i > 0 {
			val = strings.TrimSpace(val[:i])
		}
		m[key] = val
	}
	return m
}

// Current returns the slug of the active theme, or "" when none is recorded.
func Current(cfg *manifest.Themes, env inventory.Environment, tgt target.Target) string {
	if cfg == nil || cfg.NameFile == "" {
		return ""
	}
	data, err := os.ReadFile(target.ExpandPath(tgt, env, cfg.NameFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// RunSetCommand runs the owner's full theme command (for example
// omakub-theme-set), used with --full on systems that have one.
func RunSetCommand(ctx context.Context, cfg *manifest.Themes, slug string, out io.Writer) error {
	if cfg == nil || cfg.SetCommand == "" {
		return fmt.Errorf("no theme set command configured")
	}
	cmd := exec.CommandContext(ctx, cfg.SetCommand, slug)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", cfg.SetCommand, slug, err)
	}
	return nil
}

// Apply writes the chosen theme's files for the target from kit's bundled
// themes: the Zellij theme into Zellij's themes directory, and the rendered
// Alacritty theme where the Alacritty config imports its theme from. This is
// OS-independent and does not require Omakub.
func Apply(m *manifest.Manifest, slug string, env inventory.Environment, tgt target.Target, st *state.Store, out io.Writer) error {
	applied := 0

	if comp, ok := m.Find("zellij"); ok && len(comp.Configs) > 0 {
		if cfgDest := target.ExpandPath(tgt, env, comp.Configs[0].Dest[string(tgt)]); cfgDest != "" {
			data, err := ZellijFile(slug)
			if err != nil {
				return fmt.Errorf("zellij theme %q: %w", slug, err)
			}
			dest := filepath.Join(filepath.Dir(cfgDest), "themes", "current.kdl")
			if err := writeManaged(st, dest, data, out); err != nil {
				return err
			}
			applied++
		}
	}

	if comp, ok := m.Find("alacritty"); ok && len(comp.Configs) > 0 {
		if cfgDest := target.ExpandPath(tgt, env, comp.Configs[0].Dest[string(tgt)]); cfgDest != "" {
			data, err := RenderAlacritty(slug)
			if err != nil {
				return fmt.Errorf("alacritty theme %q: %w", slug, err)
			}
			dest := alacrittyThemeDest(cfgDest, env, tgt)
			if err := writeManaged(st, dest, data, out); err != nil {
				return err
			}
			applied++
		}
	}

	if applied == 0 {
		return fmt.Errorf("no zellij or alacritty destination for target %s", tgt)
	}
	return nil
}

// SetCurrent records the active theme slug in the configured name file.
func SetCurrent(cfg *manifest.Themes, env inventory.Environment, tgt target.Target, slug string) error {
	if cfg == nil || cfg.NameFile == "" {
		return nil
	}
	p := target.ExpandPath(tgt, env, cfg.NameFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(slug+"\n"), 0o644)
}

var importRe = regexp.MustCompile(`import\s*=\s*\[\s*"([^"]+)"`)

// alacrittyThemeDest returns the path the Alacritty config imports its theme
// from, falling back to a kit-owned file next to the config.
func alacrittyThemeDest(configPath string, env inventory.Environment, tgt target.Target) string {
	if data, err := os.ReadFile(configPath); err == nil {
		if mm := importRe.FindSubmatch(data); mm != nil {
			return target.ExpandPath(tgt, env, string(mm[1]))
		}
	}
	return filepath.Join(filepath.Dir(configPath), "kit-theme.toml")
}

// writeManaged backs up an existing file, writes the content, and records the
// write in the state store.
func writeManaged(st *state.Store, dest string, data []byte, out io.Writer) error {
	if st != nil {
		if _, err := st.BackupExisting(dest); err != nil {
			fmt.Fprintf(out, "warning: backup %s: %v\n", dest, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}
	if st != nil {
		st.Record(state.Entry{
			Path:      dest,
			Component: "theme",
			Source:    "theme",
			Hash:      state.Hash(data),
			WrittenAt: time.Now().Format(time.RFC3339),
		})
	}
	fmt.Fprintf(out, "  wrote %s\n", dest)
	return nil
}

// Normalize converts a user-supplied theme name into the slug form the owner
// command expects: lowercase with dashes.
func Normalize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// Label renders a slug as a readable label (tokyo-night -> Tokyo Night).
func Label(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	if len(parts) == 0 {
		return slug
	}
	return strings.Join(parts, " ")
}
