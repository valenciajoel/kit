package manifest

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var validTargets = map[string]bool{"linux": true, "wsl": true, "windows": true}

var validMethods = map[string]bool{
	"apt":    true,
	"mise":   true,
	"npm":    true,
	"script": true,
	"winget": true,
	"cargo":  true,
	"manual": true,
}

// Load reads, parses, and validates a manifest from fsys at path.
func Load(fsys fs.FS, path string) (*Manifest, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks structural invariants of the manifest.
func (m *Manifest) Validate() error {
	if m.Version != 1 {
		return fmt.Errorf("unsupported manifest version %d (want 1)", m.Version)
	}
	if len(m.Components) == 0 {
		return fmt.Errorf("manifest has no components")
	}
	seen := make(map[string]bool, len(m.Components))
	for i := range m.Components {
		c := &m.Components[i]
		if c.ID == "" {
			return fmt.Errorf("component at index %d is missing an id", i)
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate component id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Name == "" {
			return fmt.Errorf("component %q is missing a name", c.ID)
		}
		if c.Kind == "" {
			return fmt.Errorf("component %q is missing a kind", c.ID)
		}
		for j, cf := range c.Configs {
			if cf.Src == "" {
				return fmt.Errorf("component %q config[%d] is missing src", c.ID, j)
			}
			if len(cf.Dest) == 0 {
				return fmt.Errorf("component %q config[%d] has no destinations", c.ID, j)
			}
			for tgt := range cf.Dest {
				if !validTargets[tgt] {
					return fmt.Errorf("component %q config[%d] has unknown target %q", c.ID, j, tgt)
				}
			}
		}
		for tgt, ins := range c.Install {
			if !validTargets[tgt] {
				return fmt.Errorf("component %q install has unknown target %q", c.ID, tgt)
			}
			if !validMethods[ins.Method] {
				return fmt.Errorf("component %q install[%s] has unknown method %q", c.ID, tgt, ins.Method)
			}
		}
		for j, rw := range c.Rewrites {
			if rw.From == "" {
				return fmt.Errorf("component %q rewrite[%d] is missing from", c.ID, j)
			}
			if len(rw.To) == 0 {
				return fmt.Errorf("component %q rewrite[%d] has no destinations", c.ID, j)
			}
			for tgt := range rw.To {
				if !validTargets[tgt] {
					return fmt.Errorf("component %q rewrite[%d] has unknown target %q", c.ID, j, tgt)
				}
			}
		}
	}
	for name, ids := range m.Presets {
		if name == "" {
			return fmt.Errorf("preset with empty name")
		}
		for _, id := range ids {
			if !seen[id] {
				return fmt.Errorf("preset %q references unknown component %q", name, id)
			}
		}
	}
	return nil
}

// ResolvePreset returns the component ids for a named preset.
func (m *Manifest) ResolvePreset(name string) ([]string, error) {
	ids, ok := m.Presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(m.PresetNames(), ", "))
	}
	return ids, nil
}

// PresetNames returns every preset name, sorted for stable output.
func (m *Manifest) PresetNames() []string {
	names := make([]string, 0, len(m.Presets))
	for name := range m.Presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Find returns the component with the given id.
func (m *Manifest) Find(id string) (Component, bool) {
	for _, c := range m.Components {
		if c.ID == id {
			return c, true
		}
	}
	return Component{}, false
}

// Binary returns the executable used to detect a component.
func (m *Manifest) Binary(c Component) string {
	if c.Bin != "" {
		return c.Bin
	}
	return c.ID
}

// IDs returns every component id, sorted for stable output.
func (m *Manifest) IDs() []string {
	ids := make([]string, 0, len(m.Components))
	for _, c := range m.Components {
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return ids
}
