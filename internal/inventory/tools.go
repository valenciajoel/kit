package inventory

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/valenciajoel/kit/internal/manifest"
)

// ToolStatus is the detection result for a single manifest component.
type ToolStatus struct {
	ID      string
	Name    string
	Present bool
	Version string
	Path    string
	Manager string
}

// DetectTools checks every manifest component for the given target and reports
// presence, path, version, and the install method declared for that target.
func DetectTools(m *manifest.Manifest, target string) []ToolStatus {
	out := make([]ToolStatus, 0, len(m.Components))
	for _, c := range m.Components {
		bin := m.Binary(c)
		st := ToolStatus{ID: c.ID, Name: c.Name}
		if ins, ok := c.Install[target]; ok {
			st.Manager = ins.Method
		}
		if path, err := exec.LookPath(bin); err == nil {
			st.Present = true
			st.Path = path
			st.Version = probeVersion(bin)
		}
		out = append(out, st)
	}
	return out
}

// probeVersion runs `<bin> --version` with a short timeout and returns the first
// line, tolerating tools that do not support the flag.
func probeVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return ""
	}
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	if len(line) > 60 {
		line = line[:60]
	}
	return line
}
