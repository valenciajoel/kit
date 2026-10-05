// Package target models where a given machine's kit lives: native Linux, Linux
// inside WSL, or native Windows. It owns path resolution per target.
package target

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/valenciajoel/kit/internal/inventory"
)

// Target identifies the environment a kit is installed into.
type Target string

const (
	Linux   Target = "linux"
	WSL     Target = "wsl"
	Windows Target = "windows"
)

// Detect derives the target from a detected environment.
func Detect(env inventory.Environment) Target {
	switch {
	case env.OS == inventory.Windows:
		return Windows
	case env.IsWSL:
		return WSL
	default:
		return Linux
	}
}

// Valid reports whether t is a known target.
func (t Target) Valid() bool {
	switch t {
	case Linux, WSL, Windows:
		return true
	default:
		return false
	}
}

func (t Target) String() string { return string(t) }

// ExpandPath resolves `~` and Windows-style `%VAR%` placeholders against the
// detected environment, so the same manifest entry works on every target.
func ExpandPath(t Target, env inventory.Environment, p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~") {
		rest := strings.TrimPrefix(p, "~")
		rest = strings.TrimPrefix(rest, "/")
		rest = strings.TrimPrefix(rest, `\`)
		p = filepath.Join(env.Home, rest)
	}
	vars := map[string]string{
		"%APPDATA%":      os.Getenv("APPDATA"),
		"%LOCALAPPDATA%": os.Getenv("LOCALAPPDATA"),
		"%USERPROFILE%":  env.Home,
	}
	for key, val := range vars {
		if val != "" {
			p = strings.ReplaceAll(p, key, val)
		}
	}
	return p
}
