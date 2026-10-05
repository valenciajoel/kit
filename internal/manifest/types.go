package manifest

// Manifest is the top-level declarative kit definition loaded from kit.yaml.
type Manifest struct {
	Version    int         `yaml:"version"`
	Components []Component `yaml:"components"`
}

// Component describes a single tool in the kit: how to detect it, which config
// files it owns, and how to install it per target.
type Component struct {
	ID       string             `yaml:"id"`
	Name     string             `yaml:"name"`
	Kind     string             `yaml:"kind"`
	Bin      string             `yaml:"bin,omitempty"`
	Optional bool               `yaml:"optional,omitempty"`
	Configs  []ConfigSpec       `yaml:"configs,omitempty"`
	Install  map[string]Install `yaml:"install,omitempty"`
}

// ConfigSpec maps a source path on the live machine to a per-target destination.
type ConfigSpec struct {
	Src  string            `yaml:"src"`
	Dest map[string]string `yaml:"dest"`
}

// Install describes how to install a component for a given target.
type Install struct {
	Method  string `yaml:"method"`
	Pkg     string `yaml:"pkg,omitempty"`
	Tool    string `yaml:"tool,omitempty"`
	Version string `yaml:"version,omitempty"`
	URL     string `yaml:"url,omitempty"`
	Command string `yaml:"command,omitempty"`
}
