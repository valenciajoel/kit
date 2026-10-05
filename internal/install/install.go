// Package install turns the manifest into an ordered, reviewable installation
// plan and can execute it against a target.
package install

import (
	"fmt"
	"strings"

	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

// Step is one installation action for a component.
type Step struct {
	ComponentID string
	Name        string
	Method      string
	Program     string
	Args        []string
	Privileged  bool
	Skip        bool
	Reason      string
}

// Plan is the ordered set of steps for a target.
type Plan struct {
	Target  target.Target
	Steps   []Step
	Skipped []Step
}

// BuildPlan maps every manifest component to an install step for the target.
// When only is non-empty, it restricts the plan to those component ids.
func BuildPlan(m *manifest.Manifest, tgt target.Target, only []string) Plan {
	p := Plan{Target: tgt}
	want := make(map[string]bool, len(only))
	for _, id := range only {
		want[id] = true
	}

	for _, c := range m.Components {
		if len(want) > 0 && !want[c.ID] {
			continue
		}
		ins, ok := c.Install[string(tgt)]
		if !ok {
			p.Skipped = append(p.Skipped, Step{
				ComponentID: c.ID,
				Name:        c.Name,
				Skip:        true,
				Reason:      "no install entry for target " + string(tgt),
			})
			continue
		}
		step, err := buildStep(c, ins)
		if err != nil {
			p.Skipped = append(p.Skipped, Step{
				ComponentID: c.ID,
				Name:        c.Name,
				Method:      ins.Method,
				Skip:        true,
				Reason:      err.Error(),
			})
			continue
		}
		if step.Skip {
			p.Skipped = append(p.Skipped, step)
			continue
		}
		p.Steps = append(p.Steps, step)
	}
	return p
}

func buildStep(c manifest.Component, ins manifest.Install) (Step, error) {
	s := Step{ComponentID: c.ID, Name: c.Name, Method: ins.Method}

	switch ins.Method {
	case "apt":
		if ins.Pkg == "" {
			return s, fmt.Errorf("apt method requires pkg")
		}
		s.Program, s.Args, s.Privileged = "sudo", []string{"apt-get", "install", "-y", ins.Pkg}, true
	case "mise":
		tool := ins.Tool
		if tool == "" {
			tool = c.ID
		}
		ver := ins.Version
		if ver == "" {
			ver = "latest"
		}
		s.Program, s.Args = "mise", []string{"use", "-g", tool + "@" + ver}
	case "npm":
		if ins.Pkg == "" {
			return s, fmt.Errorf("npm method requires pkg")
		}
		s.Program, s.Args = "npm", []string{"install", "-g", ins.Pkg}
	case "script":
		if ins.URL == "" {
			return s, fmt.Errorf("script method requires url")
		}
		s.Program, s.Args = "sh", []string{"-c", "curl -fsSL " + ins.URL + " | sh"}
	case "winget":
		if ins.Pkg == "" {
			return s, fmt.Errorf("winget method requires pkg")
		}
		s.Program = "winget"
		s.Args = []string{"install", "--id", ins.Pkg, "-e", "--accept-source-agreements", "--accept-package-agreements"}
	case "cargo":
		pkg := ins.Pkg
		if pkg == "" {
			pkg = c.ID
		}
		s.Program, s.Args = "cargo", []string{"install", pkg}
	case "manual":
		s.Skip = true
		s.Reason = ins.Command
		if s.Reason == "" {
			s.Reason = "manual install required"
		}
	default:
		return s, fmt.Errorf("unknown method %q", ins.Method)
	}
	return s, nil
}

// Describe renders a step as a single human-readable line.
func Describe(s Step) string {
	if s.Skip {
		return fmt.Sprintf("skip   %-9s %s", s.ComponentID, s.Reason)
	}
	return fmt.Sprintf("run    %-9s %s %s", s.ComponentID, s.Program, strings.Join(s.Args, " "))
}

// Summary renders the whole plan as text, for the TUI and CLI.
func Summary(p Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "install plan (target=%s): %d run, %d skipped\n", p.Target, len(p.Steps), len(p.Skipped))
	for _, s := range p.Steps {
		b.WriteString(Describe(s) + "\n")
	}
	for _, s := range p.Skipped {
		b.WriteString(Describe(s) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
