// Package tui implements the Bubbletea interface for the kit manager.
package tui

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/bundle"
	"github.com/valenciajoel/kit/internal/install"
	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/internal/update"
)

type tab int

const (
	tabDashboard tab = iota
	tabInventory
	tabInstall
	tabActions
	tabCount
)

var tabNames = []string{"Dashboard", "Inventory", "Install", "Actions"}

// Model is the root Bubbletea model.
type Model struct {
	m          *manifest.Manifest
	env        inventory.Environment
	tgt        target.Target
	tools      []inventory.ToolStatus
	components []string
	selected   map[string]bool
	cursor     int
	presetIdx  int
	requested  bool
	version    string
	latest     string
	width      int
	height     int
	tab        tab
	status     string
	output     string
	ready      bool
}

// actionResultMsg carries the text produced by an asynchronous action.
type actionResultMsg struct {
	text   string
	status string
}

// updateResultMsg reports a newer release, if one was found.
type updateResultMsg struct {
	latest string
}

// New builds the root model and performs an initial inventory scan.
func New(m *manifest.Manifest, env inventory.Environment, tgt target.Target, version string) Model {
	components := make([]string, 0, len(m.Components))
	selected := make(map[string]bool, len(m.Components))
	for _, c := range m.Components {
		components = append(components, c.ID)
		selected[c.ID] = true
	}
	return Model{
		m:          m,
		env:        env,
		tgt:        tgt,
		tools:      inventory.DetectTools(m, string(tgt)),
		components: components,
		selected:   selected,
		presetIdx:  -1,
		version:    version,
		status:     fmt.Sprintf("loaded manifest with %d components", len(m.Components)),
	}
}

// Init implements tea.Model. It kicks off a non-blocking update check.
func (model Model) Init() tea.Cmd { return model.checkUpdate }

// checkUpdate asks GitHub for the latest release and reports it when newer.
func (model Model) checkUpdate() tea.Msg {
	if model.version == "" || model.version == "dev" {
		return updateResultMsg{}
	}
	res, err := update.Run(update.Options{
		CurrentVersion: model.version,
		CheckOnly:      true,
		Client:         &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil || res == nil || res.UpToDate || res.Latest == "" {
		return updateResultMsg{}
	}
	return updateResultMsg{latest: res.Latest}
}

// Update implements tea.Model.
func (model Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actionResultMsg:
		model.output = msg.text
		model.status = msg.status
		return model, nil
	case updateResultMsg:
		if msg.latest != "" {
			model.latest = msg.latest
		}
		return model, nil
	case tea.WindowSizeMsg:
		model.width, model.height = msg.Width, msg.Height
		model.ready = true
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return model, tea.Quit
		}
		if model.tab == tabInstall {
			return model.updateInstall(msg)
		}
		switch msg.String() {
		case "tab", "right", "l":
			model.tab = (model.tab + 1) % tabCount
		case "shift+tab", "left", "h":
			model.tab = (model.tab + tabCount - 1) % tabCount
		case "r":
			model.tools = inventory.DetectTools(model.m, string(model.tgt))
			model.status = "rescan complete"
		case "e":
			model.tab = tabActions
			model.output = "exporting…"
			return model, model.exportCmd
		case "i":
			model.tab = tabActions
			model.output = install.Summary(install.BuildPlan(model.m, model.tgt, nil))
			model.status = "install plan (dry-run)"
		case "esc":
			model.tab = tabDashboard
		}
	}
	return model, nil
}

// updateInstall handles keys on the Install tab: navigate, toggle, presets.
func (model Model) updateInstall(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if model.cursor > 0 {
			model.cursor--
		}
	case "down", "j":
		if model.cursor < len(model.components)-1 {
			model.cursor++
		}
	case " ", "x":
		id := model.components[model.cursor]
		model.selected[id] = !model.selected[id]
		model.presetIdx = -1
	case "a":
		for _, id := range model.components {
			model.selected[id] = true
		}
		model.presetIdx = -1
	case "n":
		for _, id := range model.components {
			model.selected[id] = false
		}
		model.presetIdx = -1
	case "p":
		model.cyclePreset(1)
	case "P":
		model.cyclePreset(-1)
	case "enter":
		if model.countSelected() == 0 {
			model.status = "select at least one component"
			return model, nil
		}
		model.requested = true
		return model, tea.Quit
	case "tab", "right", "l":
		model.tab = (model.tab + 1) % tabCount
	case "shift+tab", "left", "h":
		model.tab = (model.tab + tabCount - 1) % tabCount
	case "esc":
		model.tab = tabDashboard
	}
	return model, nil
}

// Selected returns the checked component ids, in manifest order.
func (model Model) Selected() []string {
	out := make([]string, 0, len(model.components))
	for _, id := range model.components {
		if model.selected[id] {
			out = append(out, id)
		}
	}
	return out
}

// Requested reports whether the user asked to install the current selection.
func (model Model) Requested() bool { return model.requested }

func (model Model) countSelected() int {
	n := 0
	for _, id := range model.components {
		if model.selected[id] {
			n++
		}
	}
	return n
}

// cyclePreset replaces the selection with the next (or previous) preset.
func (model *Model) cyclePreset(dir int) {
	names := model.m.PresetNames()
	if len(names) == 0 {
		return
	}
	model.presetIdx = ((model.presetIdx+dir)%len(names) + len(names)) % len(names)
	ids, err := model.m.ResolvePreset(names[model.presetIdx])
	if err != nil {
		return
	}
	for k := range model.selected {
		model.selected[k] = false
	}
	for _, id := range ids {
		model.selected[id] = true
	}
	model.status = "preset: " + names[model.presetIdx]
}

// presetName returns the preset currently applied, or "" for a custom selection.
func (model Model) presetName() string {
	names := model.m.PresetNames()
	if model.presetIdx < 0 || model.presetIdx >= len(names) {
		return ""
	}
	return names[model.presetIdx]
}

// exportCmd captures the kit in the background and reports the result.
func (model Model) exportCmd() tea.Msg {
	res, err := bundle.Export(bundle.Options{
		Manifest: model.m,
		Env:      model.env,
		Target:   model.tgt,
		OutDir:   bundle.DefaultOutDir,
	})
	if err != nil {
		return actionResultMsg{text: "export failed: " + err.Error(), status: "export failed"}
	}
	text := fmt.Sprintf("exported %d file(s), %d redaction(s)\n%s", len(res.Captured), len(res.Redactions), res.BundlePath)
	for _, w := range res.Warnings {
		text += "\nwarning: " + w
	}
	return actionResultMsg{text: text, status: "export complete"}
}

// View implements tea.Model.
func (model Model) View() string {
	if !model.ready {
		return "loading…"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("kit — terminal kit manager"))
	b.WriteString("\n")
	b.WriteString(model.renderUpdateNotice())
	b.WriteString(model.renderTabs())
	b.WriteString("\n\n")

	switch model.tab {
	case tabDashboard:
		b.WriteString(model.renderDashboard())
	case tabInventory:
		b.WriteString(model.renderInventory())
	case tabInstall:
		b.WriteString(model.renderInstall())
	case tabActions:
		b.WriteString(model.renderActions())
	}

	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render(model.status))
	b.WriteString("\n")
	b.WriteString(model.renderFooter())
	return b.String()
}

// renderUpdateNotice returns a banner when a newer release was detected, or "".
func (model Model) renderUpdateNotice() string {
	if model.latest == "" {
		return ""
	}
	return updateStyle.Render(fmt.Sprintf("⬆ update available: %s → %s   (run: kit update)", model.version, model.latest)) + "\n"
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

func (model Model) renderInstall() string {
	var b strings.Builder
	for i, id := range model.components {
		c, ok := model.m.Find(id)
		name := id
		if ok {
			name = c.Name
		}
		box, style := "[ ]", missingStyle
		if model.selected[id] {
			box, style = "[x]", okStyle
		}
		cursor := "  "
		if i == model.cursor {
			cursor = keyStyle.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %-26s %s\n", cursor, style.Render(box), name, subtleStyle.Render(id)))
	}

	b.WriteString("\n")
	if name := model.presetName(); name != "" {
		b.WriteString(keyStyle.Render("preset: ") + name)
	} else {
		b.WriteString(subtleStyle.Render("preset: custom (p cycles presets)"))
	}
	b.WriteString("   " + okStyle.Render(fmt.Sprintf("%d selected", model.countSelected())))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("space toggle · a all · n none · p preset · enter install & quit"))
	return boxStyle.Render(strings.TrimRight(b.String(), "\n"))
}

func (model Model) renderActions() string {
	lines := []string{
		keyStyle.Render("e") + "  export kit bundle   " + subtleStyle.Render("(writes to "+bundle.DefaultOutDir+"/)"),
		keyStyle.Render("i") + "  install plan       " + subtleStyle.Render("(dry-run)"),
	}
	if model.output != "" {
		lines = append(lines, "", model.output)
	}
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (model Model) renderFooter() string {
	items := []string{
		keyStyle.Render("tab/←/→") + subtleStyle.Render(" switch"),
		keyStyle.Render("e") + subtleStyle.Render(" export"),
		keyStyle.Render("i") + subtleStyle.Render(" install"),
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
