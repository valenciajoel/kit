// Package tui implements the Bubbletea interface for the kit manager.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
)

type tab int

const (
	tabDashboard tab = iota
	tabInventory
	tabActions
	tabCount
)

var tabNames = []string{"Dashboard", "Inventory", "Actions"}

// Model is the root Bubbletea model.
type Model struct {
	m      *manifest.Manifest
	env    inventory.Environment
	tgt    target.Target
	tools  []inventory.ToolStatus
	width  int
	height int
	tab    tab
	status string
	ready  bool
}

// New builds the root model and performs an initial inventory scan.
func New(m *manifest.Manifest, env inventory.Environment, tgt target.Target) Model {
	return Model{
		m:      m,
		env:    env,
		tgt:    tgt,
		tools:  inventory.DetectTools(m, string(tgt)),
		status: "loaded manifest with " + fmt.Sprintf("%d", len(m.Components)) + " components",
	}
}

// Init implements tea.Model.
func (model Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (model Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.width, model.height = msg.Width, msg.Height
		model.ready = true
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return model, tea.Quit
		case "tab", "right", "l":
			model.tab = (model.tab + 1) % tabCount
		case "shift+tab", "left", "h":
			model.tab = (model.tab + tabCount - 1) % tabCount
		case "r":
			model.tools = inventory.DetectTools(model.m, string(model.tgt))
			model.status = "rescan complete"
		case "esc":
			model.tab = tabDashboard
		}
	}
	return model, nil
}

// View implements tea.Model.
func (model Model) View() string {
	if !model.ready {
		return "loading…"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("kit — terminal kit manager"))
	b.WriteString("\n")
	b.WriteString(model.renderTabs())
	b.WriteString("\n\n")

	switch model.tab {
	case tabDashboard:
		b.WriteString(model.renderDashboard())
	case tabInventory:
		b.WriteString(model.renderInventory())
	case tabActions:
		b.WriteString(model.renderActions())
	}

	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render(model.status))
	b.WriteString("\n")
	b.WriteString(model.renderFooter())
	return b.String()
}

func (model Model) renderTabs() string {
	parts := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if tab(i) == model.tab {
			parts = append(parts, activeTabStyle.Render(" "+name+" "))
		} else {
			parts = append(parts, inactiveTabStyle.Render(" "+name+" "))
		}
	}
	return strings.Join(parts, " ")
}

func (model Model) renderDashboard() string {
	rows := [][2]string{
		{"OS", string(model.env.OS)},
		{"Arch", model.env.Arch},
		{"Target", string(model.tgt)},
		{"WSL", boolWord(model.env.IsWSL)},
		{"Distro", strings.TrimSpace(model.env.Distro + " " + model.env.DistroVersion)},
		{"Shell", model.env.Shell},
		{"Home", model.env.Home},
		{"Config root", model.env.ConfigDir},
		{"Host", model.env.Hostname},
	}

	var b strings.Builder
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%-14s %s\n", subtleStyle.Render(r[0]), r[1]))
	}

	present, total := 0, len(model.tools)
	for _, t := range model.tools {
		if t.Present {
			present++
		}
	}
	b.WriteString("\n")
	b.WriteString(keyStyle.Render("Components") + "  ")
	b.WriteString(okStyle.Render(fmt.Sprintf("%d/%d present", present, total)))
	return boxStyle.Render(strings.TrimRight(b.String(), "\n"))
}

func (model Model) renderInventory() string {
	var b strings.Builder
	for _, t := range model.tools {
		mark := missingStyle.Render("✗")
		version := ""
		if t.Present {
			mark = okStyle.Render("✓")
			version = subtleStyle.Render(t.Version)
		}
		b.WriteString(fmt.Sprintf("%s %-24s %-14s %s\n", mark, t.Name, subtleStyle.Render(t.Manager), version))
	}
	return boxStyle.Render(strings.TrimRight(b.String(), "\n"))
}

func (model Model) renderActions() string {
	lines := []string{
		keyStyle.Render("e") + "  export kit bundle    " + subtleStyle.Render("(planned: T7)"),
		keyStyle.Render("i") + "  install on target    " + subtleStyle.Render("(planned: T8)"),
		keyStyle.Render("d") + "  dry-run evaluation   " + subtleStyle.Render("(planned: T8)"),
	}
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (model Model) renderFooter() string {
	items := []string{
		keyStyle.Render("tab/←/→") + subtleStyle.Render(" switch"),
		keyStyle.Render("r") + subtleStyle.Render(" rescan"),
		keyStyle.Render("q") + subtleStyle.Render(" quit"),
	}
	return subtleStyle.Render(strings.Join(items, "   "))
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
