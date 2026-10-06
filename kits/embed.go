// Package kits embeds the default kit manifest and seed configuration files so
// the tool works without any external assets checked out on disk.
package kits

import "embed"

// FS holds the default manifest (kit.yaml) and the seed configs directory.
//
//go:embed kit.yaml configs themes
var FS embed.FS
