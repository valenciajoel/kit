package install

import (
	"testing"

	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{Version: 1, Components: []manifest.Component{
		{ID: "zellij", Name: "Zellij", Kind: "terminal", Install: map[string]manifest.Install{"linux": {Method: "apt", Pkg: "zellij"}}},
		{ID: "go", Name: "Go", Kind: "language", Install: map[string]manifest.Install{"linux": {Method: "mise", Tool: "go", Version: "latest"}}},
		{ID: "opencode", Name: "opencode", Kind: "ai", Install: map[string]manifest.Install{"linux": {Method: "npm", Pkg: "opencode-ai"}}},
		{ID: "font", Name: "Font", Kind: "font", Install: map[string]manifest.Install{"linux": {Method: "manual"}}},
		{ID: "winonly", Name: "WinOnly", Kind: "x", Install: map[string]manifest.Install{"windows": {Method: "winget", Pkg: "X"}}},
	}}
}

func TestBuildPlanMethods(t *testing.T) {
	p := BuildPlan(testManifest(), target.Linux, nil)

	if len(p.Steps) != 3 {
		t.Fatalf("want 3 steps, got %d", len(p.Steps))
	}
	if len(p.Skipped) != 2 {
		t.Fatalf("want 2 skipped, got %d", len(p.Skipped))
	}

	byID := make(map[string]Step, len(p.Steps))
	for _, s := range p.Steps {
		byID[s.ComponentID] = s
	}
	if byID["zellij"].Program != "sudo" || !byID["zellij"].Privileged {
		t.Errorf("apt step wrong: %+v", byID["zellij"])
	}
	if byID["go"].Program != "mise" || last(byID["go"].Args) != "go@latest" {
		t.Errorf("mise step wrong: %+v", byID["go"])
	}
	if byID["opencode"].Program != "npm" || last(byID["opencode"].Args) != "opencode-ai" {
		t.Errorf("npm step wrong: %+v", byID["opencode"])
	}
}

func last(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func TestBuildPlanOnlyFilter(t *testing.T) {
	p := BuildPlan(testManifest(), target.Linux, []string{"go"})
	if len(p.Steps) != 1 || p.Steps[0].ComponentID != "go" {
		t.Fatalf("filter failed: %+v", p.Steps)
	}
}

func TestBuildPlanManualIsSkipped(t *testing.T) {
	p := BuildPlan(testManifest(), target.Linux, []string{"font"})
	if len(p.Steps) != 0 {
		t.Fatalf("manual should not run, got %+v", p.Steps)
	}
	if len(p.Skipped) != 1 || !p.Skipped[0].Skip {
		t.Fatalf("manual should be skipped: %+v", p.Skipped)
	}
}
