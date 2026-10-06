// Package theme discovers the themes available on this machine and applies a
// chosen one by delegating to the owner's own command (for example Omakub's
// omakub-theme-set), so kit never reimplements theme rendering.
package theme

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

// Theme is one selectable theme.
type Theme struct {
	Slug  string
	Label string
}

// List returns the themes available on this machine, sorted by label. A missing
// theme directory yields an empty list, not an error.
func List(cfg *manifest.Themes, env inventory.Environment, tgt target.Target) ([]Theme, error) {
	if cfg == nil || cfg.Dir == "" {
		return nil, nil
	}
	dir := target.ExpandPath(tgt, env, cfg.Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Theme
	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		out = append(out, Theme{Slug: e.Name(), Label: Label(e.Name())})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out, nil
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

// Apply sets the theme by running the configured command with the slug.
func Apply(ctx context.Context, cfg *manifest.Themes, slug string, out io.Writer) error {
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
