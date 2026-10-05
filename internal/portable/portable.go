// Package portable rewrites machine-specific paths so captured configuration
// survives a move between machines or targets.
package portable

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/valenciajoel/kit/internal/inventory"
)

// Normalize replaces the current machine's home directory with `~`, so an
// exported config never carries an absolute path or an account name.
func Normalize(data []byte, env inventory.Environment) []byte {
	for _, variant := range homeVariants(env.Home) {
		data = bytes.ReplaceAll(data, []byte(variant), []byte("~"))
	}
	return data
}

// Render replaces `~` with the home directory of the machine the config is
// being restored onto, using forward slashes for cross-tool friendliness.
func Render(data []byte, env inventory.Environment) []byte {
	if env.Home == "" {
		return data
	}
	home := filepath.ToSlash(env.Home)
	return bytes.ReplaceAll(data, []byte("~"), []byte(home))
}

// homeVariants returns the ways a home path may appear in config text, longest
// first so escaped forms are replaced before their unescaped prefixes.
func homeVariants(home string) []string {
	if home == "" {
		return nil
	}
	// Normalize separators first so this works regardless of the OS the export
	// runs on (filepath.ToSlash only rewrites the host separator).
	slashed := strings.ReplaceAll(home, `\`, "/")
	candidates := []string{
		strings.ReplaceAll(slashed, "/", `\\`), // JSON-escaped backslashes
		strings.ReplaceAll(slashed, "/", `\`),  // raw Windows path
		slashed,                                // forward slashes
		home,                                   // native form
	}
	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if len(candidates[j]) > len(candidates[i]) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
