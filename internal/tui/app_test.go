package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Version: 1,
		Components: []manifest.Component{
			{ID: "zellij", Name: "Zellij", Kind: "terminal"},
			{ID: "go", Name: "Go", Kind: "language"},
			{ID: "nvim", Name: "Neovim", Kind: "editor"},
		},
		Presets: map[string][]string{"pair": {"zellij", "go"}},
	}
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(m Model, key string) Model {
	updated, _ := m.Update(keyMsg(key))
	return updated.(Model)
}

func newInstallModel() Model {
	m := New(testManifest(), inventory.Environment{OS: inventory.Linux}, target.Linux, "v0.1.7")
	m.tab = tabInstall
	return m
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestDefaultSelectsAll(t *testing.T) {
	m := newInstallModel()
	if got := m.Selected(); len(got) != 3 {
		t.Fatalf("default selection = %v, want all 3", got)
	}
}

func TestToggleAndRequest(t *testing.T) {
	m := newInstallModel()
	m = press(m, " ") // toggle first off
	if got := m.Selected(); len(got) != 2 {
		t.Fatalf("after toggle selection = %v, want 2", got)
	}
	m = press(m, "enter")
	if !m.Requested() {
		t.Fatal("enter with a non-empty selection should request install")
	}
}

func TestEmptySelectionDoesNotRequest(t *testing.T) {
	m := newInstallModel()
	m = press(m, "n") // clear all
	m = press(m, "enter")
	if m.Requested() {
		t.Fatal("enter with an empty selection must not request install")
	}
}

func TestUpdateNotice(t *testing.T) {
	m := New(testManifest(), inventory.Environment{OS: inventory.Linux}, target.Linux, "v0.1.7")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	if strings.Contains(m.View(), "update available") {
		t.Fatal("no notice expected before an update result arrives")
	}

	updated, _ = m.Update(updateResultMsg{latest: "v9.9.9"})
	m = updated.(Model)
	if !strings.Contains(m.View(), "update available") {
		t.Fatal("expected an update notice after an update result")
	}
}

func TestPresetCycling(t *testing.T) {
	m := newInstallModel()
	m = press(m, "p") // apply preset "pair"
	got := m.Selected()
	if len(got) != 2 || !contains(got, "zellij") || !contains(got, "go") {
		t.Fatalf("preset selection = %v, want [zellij go]", got)
	}
	if m.presetName() != "pair" {
		t.Fatalf("preset name = %q, want pair", m.presetName())
	}
}
