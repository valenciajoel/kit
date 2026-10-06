package manifest

// Manifest is the top-level declarative kit definition loaded from kit.yaml.
type Manifest struct {
	Version    int                 `yaml:"version"`
	Components []Component         `yaml:"components"`
	Presets    map[string][]string `yaml:"presets,omitempty"`
	Themes     *Themes             `yaml:"themes,omitempty"`
}

// Themes points kit at the theme system that owns the machine's themes, so kit
// can list them and delegate applying them without reimplementing rendering.
type Themes struct {
	Dir        string `yaml:"dir"`
	NameFile   string `yaml:"name_file,omitempty"`
	SetCommand string `yaml:"set_command"`
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
	Rewrites []Rewrite          `yaml:"rewrites,omitempty"`
	Install  map[string]Install `yaml:"install,omitempty"`
	Seed     string             `yaml:"seed,omitempty"`
}

// Rewrite points a config reference at a bundled path, so third-party imports
// (for example the Omakub alacritty theme) survive a move to another machine.
type Rewrite struct {
	From   string            `yaml:"from"`
	To     map[string]string `yaml:"to"`
	Bundle bool              `yaml:"bundle,omitempty"`
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
