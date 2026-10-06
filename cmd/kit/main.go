// Command kit is a cross-platform TUI that detects, exports, and installs the
// user's terminal/dev kit (Zellij + Alacritty + nvim + starship + mise + opencode).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/bundle"
	"github.com/valenciajoel/kit/internal/install"
	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/restore"
	"github.com/valenciajoel/kit/internal/state"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/internal/theme"
	"github.com/valenciajoel/kit/internal/tui"
	"github.com/valenciajoel/kit/internal/update"
	"github.com/valenciajoel/kit/kits"
)

// version is overridable at build time via -ldflags "-X main.version=...".
// It stays "dev" for `go install`, where no ldflags are applied.
var version = "dev"

// currentVersion returns the ldflags-injected version when present, otherwise
// the module version reported by the Go toolchain (so `go install` builds
// report their real version instead of "dev").
func currentVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	version = currentVersion()

	m, err := manifest.Load(kits.FS, "kit.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit: "+err.Error())
		os.Exit(1)
	}

	env := inventory.DetectEnv()
	tgt := target.Detect(env)

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "detect":
			cmdDetect(m, env, tgt)
		case "export":
			cmdExport(m, env, tgt, args[1:])
		case "install":
			cmdInstall(m, env, tgt, args[1:])
		case "restore":
			cmdRestore(m, env, tgt, args[1:])
		case "setup":
			cmdSetup(m, env, tgt, args[1:])
		case "presets":
			cmdPresets(m)
		case "update":
			cmdUpdate(args[1:])
		case "themes":
			cmdThemes(m, env, tgt)
		case "theme":
			cmdTheme(m, env, tgt, args[1:])
		case "state":
			cmdState(env)
		case "uninstall":
			cmdUninstall(env, args[1:])
		case "version", "--version", "-v":
			fmt.Println("kit " + version)
		case "help", "--help", "-h":
			usage()
		default:
			fmt.Fprintf(os.Stderr, "kit: unknown command %q\n\n", args[0])
			usage()
			os.Exit(2)
		}
		return
	}

	program := tea.NewProgram(tui.New(m, env, tgt, version), tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit: "+err.Error())
		os.Exit(1)
	}
	if fm, ok := final.(tui.Model); ok && fm.Requested() {
		only := fm.Selected()
		fmt.Printf("kit setup (target=%s, applying %d selected component(s))\n", tgt, len(only))
		if err := runSetupSelection(m, env, tgt, only); err != nil {
			fmt.Fprintln(os.Stderr, "kit: "+err.Error())
			os.Exit(1)
		}
	}
	if fm, ok := final.(tui.Model); ok && fm.RequestedTheme() != "" {
		slug := fm.RequestedTheme()
		fmt.Printf("kit theme → %s\n", slug)
		st, serr := state.Load(env)
		if serr != nil {
			fmt.Fprintln(os.Stderr, "kit: "+serr.Error())
			os.Exit(1)
		}
		if err := theme.Apply(m, slug, env, tgt, st, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "kit: "+err.Error())
			os.Exit(1)
		}
		if err := theme.SetCurrent(m.Themes, env, tgt, slug); err != nil {
			fmt.Fprintln(os.Stderr, "kit: "+err.Error())
			os.Exit(1)
		}
		_ = st.Save()
		fmt.Printf("theme set to %s\n", slug)
	}
}

// runSetupSelection installs the tools and writes the configs for the chosen
// components. It runs after the TUI exits so subprocesses get a normal terminal.
func runSetupSelection(m *manifest.Manifest, env inventory.Environment, tgt target.Target, only []string) error {
	install.AugmentPath(env)
	plan := install.BuildPlan(m, tgt, only)
	runner := install.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	if err := install.Execute(context.Background(), plan, runner, true, os.Stdout); err != nil {
		return err
	}

	st, err := state.Load(env)
	if err != nil {
		return err
	}
	res, err := restore.Restore(restore.Options{
		Manifest: m,
		Env:      env,
		Target:   tgt,
		Only:     only,
		State:    st,
	})
	if err != nil {
		return err
	}
	if err := st.Save(); err != nil {
		return err
	}
	printRestoreResult(res)
	return nil
}

func cmdDetect(m *manifest.Manifest, env inventory.Environment, tgt target.Target) {
	fmt.Printf("kit %s\n", version)
	fmt.Printf("OS          %s/%s\n", env.OS, env.Arch)
	fmt.Printf("Target      %s\n", tgt)
	fmt.Printf("WSL         %v\n", env.IsWSL)
	fmt.Printf("Distro      %s\n", strings.TrimSpace(env.Distro+" "+env.DistroVersion))
	fmt.Printf("Shell       %s\n", env.Shell)
	fmt.Printf("Home        %s\n", env.Home)
	fmt.Printf("Config root %s\n", env.ConfigDir)
	fmt.Println()
	fmt.Println("Components:")

	for _, t := range inventory.DetectTools(m, string(tgt)) {
		status := "absent"
		if t.Present {
			status = "present"
		}
		fmt.Printf("  %-9s %-8s %-22s %s\n", t.ID, status, t.Version, t.Path)
	}
}

func cmdExport(m *manifest.Manifest, env inventory.Environment, tgt target.Target, args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	out := fs.String("out", bundle.DefaultOutDir, "output directory for the bundle")
	dry := fs.Bool("dry-run", false, "report captured files and redactions without writing")
	preset := fs.String("preset", "", "preset of components")
	components := fs.String("components", "", "comma-separated component ids")
	_ = fs.Parse(args)

	only, err := resolveOnly(m, *preset, *components, fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit export: "+err.Error())
		os.Exit(2)
	}

	res, err := bundle.Export(bundle.Options{
		Manifest: m,
		Env:      env,
		Target:   tgt,
		OutDir:   *out,
		DryRun:   *dry,
		Only:     only,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit export: "+err.Error())
		os.Exit(1)
	}

	if *dry {
		fmt.Println("kit export (dry-run) — nothing written")
	} else {
		fmt.Println("kit export → " + res.BundlePath)
	}
	fmt.Printf("captured %d file(s), %d redaction(s)\n", len(res.Captured), len(res.Redactions))
	for _, c := range res.Captured {
		note := ""
		if n := len(c.Redactions); n > 0 {
			note = fmt.Sprintf("  [%d redacted]", n)
		}
		fmt.Printf("  %-42s %s%s\n", c.ArchivePath, humanBytes(c.Bytes), note)
	}
	if len(res.Redactions) > 0 {
		fmt.Println("redactions:")
		for _, r := range res.Redactions {
			loc := r.File
			if r.Line > 0 {
				loc = fmt.Sprintf("%s:%d", r.File, r.Line)
			}
			fmt.Printf("  %-58s %s\n", loc, r.Key)
		}
	}
	for _, w := range res.Warnings {
		fmt.Println("  warning: " + w)
	}
}

func cmdInstall(m *manifest.Manifest, env inventory.Environment, tgt target.Target, args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	apply := fs.Bool("apply", false, "execute the plan instead of only printing it")
	interactive := fs.Bool("interactive", false, "accept each component one by one")
	fs.BoolVar(interactive, "i", false, "shorthand for --interactive")
	preset := fs.String("preset", "", "preset of components")
	components := fs.String("components", "", "comma-separated component ids")
	_ = fs.Parse(args)

	only, err := resolveOnly(m, *preset, *components, fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit install: "+err.Error())
		os.Exit(2)
	}
	plan := install.BuildPlan(m, tgt, only)
	runner := install.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr}

	if *interactive {
		install.AugmentPath(env)
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		if err := install.RunInteractive(context.Background(), plan, runner, prompt); err != nil {
			fmt.Fprintln(os.Stderr, "kit install: "+err.Error())
			os.Exit(1)
		}
		return
	}

	if err := install.Execute(context.Background(), plan, runner, *apply, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "kit install: "+err.Error())
		os.Exit(1)
	}
}

func cmdRestore(m *manifest.Manifest, env inventory.Environment, tgt target.Target, args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "report what would be written without writing")
	force := fs.Bool("force", false, "overwrite existing files")
	targetName := fs.String("target", string(tgt), "destination target: linux|wsl|windows")
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: kit restore [--dry-run] [--force] [--target T] <bundle.zip>")
		os.Exit(2)
	}
	rt := target.Target(*targetName)
	if !rt.Valid() {
		fmt.Fprintf(os.Stderr, "kit restore: invalid target %q\n", *targetName)
		os.Exit(2)
	}

	st, err := state.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit restore: "+err.Error())
		os.Exit(1)
	}

	res, err := restore.Restore(restore.Options{
		BundlePath: fs.Arg(0),
		Manifest:   m,
		Env:        env,
		Target:     rt,
		DryRun:     *dry,
		Force:      *force,
		State:      st,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit restore: "+err.Error())
		os.Exit(1)
	}
	if !*dry {
		if err := st.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "kit restore: "+err.Error())
			os.Exit(1)
		}
	}

	if *dry {
		fmt.Printf("kit restore (dry-run, target=%s) — nothing written\n", rt)
	} else {
		fmt.Printf("kit restore (target=%s)\n", rt)
	}
	printRestoreResult(res)
}

// cmdSetup installs the tools and writes the configs in one run. It is a
// dry-run by default; pass --apply to execute, or --interactive (-i) to accept
// each component one by one.
func cmdSetup(m *manifest.Manifest, env inventory.Environment, tgt target.Target, args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	apply := fs.Bool("apply", false, "execute the setup instead of only printing it")
	interactive := fs.Bool("interactive", false, "accept tools and configs one by one")
	fs.BoolVar(interactive, "i", false, "shorthand for --interactive")
	from := fs.String("from", "", "bundle zip to restore configs from (default: seed configs)")
	force := fs.Bool("force", false, "overwrite existing config files")
	targetName := fs.String("target", string(tgt), "destination target: linux|wsl|windows")
	skipTools := fs.Bool("skip-tools", false, "do not install tools")
	skipConfigs := fs.Bool("skip-configs", false, "do not write config files")
	preset := fs.String("preset", "", "preset of components")
	components := fs.String("components", "", "comma-separated component ids")
	_ = fs.Parse(args)

	rt := target.Target(*targetName)
	if !rt.Valid() {
		fmt.Fprintf(os.Stderr, "kit setup: invalid target %q\n", *targetName)
		os.Exit(2)
	}

	only, err := resolveOnly(m, *preset, *components, fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
		os.Exit(2)
	}

	st, err := state.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
		os.Exit(1)
	}

	runner := install.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr}

	if *interactive {
		fmt.Printf("kit setup (target=%s, interactive)\n", rt)
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		install.AugmentPath(env)

		if !*skipTools {
			fmt.Println("\n== tools ==")
			plan := install.BuildPlan(m, rt, only)
			if err := install.RunInteractive(context.Background(), plan, runner, prompt); err != nil {
				fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
				os.Exit(1)
			}
		}

		if !*skipConfigs {
			fmt.Println("\n== configs ==")
			if *from == "" {
				fmt.Println("(no bundle given: seed configs)")
			}
			if prompt.Yes("write config files? [y/N] ") {
				res, err := restore.Restore(restore.Options{
					BundlePath: *from,
					Manifest:   m,
					Env:        env,
					Target:     rt,
					Force:      *force,
					Only:       only,
					State:      st,
				})
				if err != nil {
					fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
					os.Exit(1)
				}
				if err := st.Save(); err != nil {
					fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
					os.Exit(1)
				}
				printRestoreResult(res)
			} else {
				fmt.Println("configs skipped")
			}
		}
		return
	}

	mode := "dry-run — nothing will be changed"
	if *apply {
		mode = "APPLY"
	}
	fmt.Printf("kit setup (target=%s, %s)\n", rt, mode)

	if !*skipTools {
		fmt.Println("\n== tools ==")
		plan := install.BuildPlan(m, rt, only)
		if *apply {
			install.AugmentPath(env)
		}
		if err := install.Execute(context.Background(), plan, runner, *apply, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
			os.Exit(1)
		}
	}

	if !*skipConfigs {
		fmt.Println("\n== configs ==")
		if *from == "" {
			fmt.Println("(no bundle given: writing seed configs; use --from <bundle.zip> to restore yours)")
		}
		res, err := restore.Restore(restore.Options{
			BundlePath: *from,
			Manifest:   m,
			Env:        env,
			Target:     rt,
			DryRun:     !*apply,
			Force:      *force,
			Only:       only,
			State:      st,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
			os.Exit(1)
		}
		if *apply {
			if err := st.Save(); err != nil {
				fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
				os.Exit(1)
			}
		}
		printRestoreResult(res)
	}

	if !*apply {
		fmt.Println("\n(dry-run; re-run with --apply to execute)")
	}
}

func printRestoreResult(res *restore.Result) {
	fmt.Printf("wrote %d file(s), skipped %d\n", len(res.Written), len(res.Skipped))
	for _, p := range res.Written {
		fmt.Println("  + " + p)
	}
	for _, p := range res.Skipped {
		fmt.Println("  = " + p)
	}
	for _, w := range res.Warnings {
		fmt.Println("  warning: " + w)
	}
}

// cmdUpdate checks GitHub releases and self-replaces the binary when newer.
func cmdUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only check for a newer release")
	force := fs.Bool("force", false, "reinstall even if already current")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	fs.BoolVar(yes, "y", false, "shorthand for --yes")
	_ = fs.Parse(args)

	res, err := update.Run(update.Options{CurrentVersion: version, CheckOnly: true, Force: *force})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit update: "+err.Error())
		os.Exit(1)
	}
	if res.UpToDate {
		fmt.Printf("kit %s is up to date\n", res.Current)
		return
	}
	fmt.Printf("kit %s → %s available\n", res.Current, res.Latest)
	if *check {
		return
	}
	if !*yes {
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		if !prompt.Yes("update now? [y/N] ") {
			fmt.Println("aborted")
			return
		}
	}

	res2, err := update.Run(update.Options{CurrentVersion: version, Force: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit update: "+err.Error())
		os.Exit(1)
	}
	fmt.Printf("updated to %s\n", res2.Latest)
}

func cmdPresets(m *manifest.Manifest) {
	if len(m.Presets) == 0 {
		fmt.Println("(no presets defined)")
		return
	}
	for _, name := range m.PresetNames() {
		ids, _ := m.ResolvePreset(name)
		fmt.Printf("%-11s %s\n", name, strings.Join(ids, ", "))
	}
}

// cmdThemes lists the themes available on this machine, marking the active one.
func cmdThemes(m *manifest.Manifest, env inventory.Environment, tgt target.Target) {
	themes, err := theme.List(m.Themes, env, tgt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit themes: "+err.Error())
		os.Exit(1)
	}
	if len(themes) == 0 {
		fmt.Println("no themes found (set themes.dir in kit.yaml)")
		return
	}
	cur := theme.Current(m.Themes, env, tgt)
	for _, th := range themes {
		mark := "  "
		if th.Slug == cur {
			mark = "* "
		}
		fmt.Printf("%s%-18s %s\n", mark, th.Slug, th.Label)
	}
}

// cmdTheme shows or sets the active theme by delegating to the owner command.
// Flags are parsed manually so they may appear before or after the theme name.
func cmdTheme(m *manifest.Manifest, env inventory.Environment, tgt target.Target, args []string) {
	var (
		dry  bool
		yes  bool
		full bool
		name string
	)
	for _, a := range args {
		switch a {
		case "--dry-run", "-n":
			dry = true
		case "--yes", "-y":
			yes = true
		case "--full", "-f":
			full = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "kit theme: unknown flag %q\n", a)
				os.Exit(2)
			}
			if name != "" {
				fmt.Fprintln(os.Stderr, "kit theme: expected at most one theme name")
				os.Exit(2)
			}
			name = a
		}
	}

	themes, err := theme.List(m.Themes, env, tgt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
		os.Exit(1)
	}
	cur := theme.Current(m.Themes, env, tgt)

	if name == "" {
		if cur == "" {
			fmt.Println("no theme set")
			return
		}
		fmt.Println(cur)
		return
	}

	slug := theme.Normalize(name)
	found := false
	for _, th := range themes {
		if th.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "kit theme: unknown theme %q (run `kit themes`)\n", slug)
		os.Exit(2)
	}
	setCmd := "the theme set command"
	if m.Themes != nil && m.Themes.SetCommand != "" {
		setCmd = m.Themes.SetCommand
	}
	if dry {
		fmt.Printf("would apply theme %q to zellij + alacritty (target=%s)\n", slug, tgt)
		if full {
			fmt.Printf("and run: %s %s\n", setCmd, slug)
		}
		return
	}
	if !yes {
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		if !prompt.Yes(fmt.Sprintf("apply theme %q? [y/N] ", slug)) {
			fmt.Println("aborted")
			return
		}
	}

	st, err := state.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
		os.Exit(1)
	}
	if err := theme.Apply(m, slug, env, tgt, st, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
		os.Exit(1)
	}
	if err := theme.SetCurrent(m.Themes, env, tgt, slug); err != nil {
		fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
		os.Exit(1)
	}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
		os.Exit(1)
	}
	if full && m.Themes != nil && m.Themes.SetCommand != "" {
		if err := theme.RunSetCommand(context.Background(), m.Themes, slug, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "kit theme: "+err.Error())
			os.Exit(1)
		}
	}
	fmt.Printf("theme set to %s\n", slug)
}

// cmdState prints what kit manages on this machine.
func cmdState(env inventory.Environment) {
	st, err := state.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit state: "+err.Error())
		os.Exit(1)
	}
	fmt.Printf("kit state — %d managed file(s)\n", st.Len())
	if st.UpdatedAt != "" {
		fmt.Printf("last updated: %s\n", st.UpdatedAt)
	}
	for _, e := range st.Files {
		note := ""
		if e.Backup != "" {
			note = "  (backup kept)"
		}
		fmt.Printf("  %-9s %s%s\n", e.Component, e.Path, note)
	}
	fmt.Printf("state file:  %s\n", state.FilePath(env))
	fmt.Printf("backups dir: %s\n", filepath.Join(state.Dir(env), "backups"))
}

// cmdUninstall removes (or restores) the files kit wrote, leaving user edits.
func cmdUninstall(env inventory.Environment, args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "report without changing anything")
	restoreBackups := fs.Bool("restore", false, "restore backups instead of deleting")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	fs.BoolVar(yes, "y", false, "shorthand for --yes")
	_ = fs.Parse(args)

	st, err := state.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit uninstall: "+err.Error())
		os.Exit(1)
	}
	if st.Len() == 0 {
		fmt.Println("kit manages no files on this machine")
		return
	}

	if !*dry && !*yes {
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		if !prompt.Yes(fmt.Sprintf("remove %d managed file(s)? [y/N] ", st.Len())) {
			fmt.Println("aborted")
			return
		}
	}

	removed, restored, kept := 0, 0, 0
	var done []string
	for _, e := range st.Files {
		if *dry {
			action := "delete"
			if *restoreBackups && e.Backup != "" {
				action = "restore"
			}
			fmt.Printf("  %-8s %s\n", action, e.Path)
			continue
		}

		if *restoreBackups && e.Backup != "" {
			data, err := os.ReadFile(e.Backup)
			if err != nil {
				fmt.Printf("  warning: %s\n", err)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(e.Path), 0o755); err != nil {
				fmt.Printf("  warning: %s\n", err)
				continue
			}
			if err := os.WriteFile(e.Path, data, 0o644); err != nil {
				fmt.Printf("  warning: %s\n", err)
				continue
			}
			fmt.Printf("  restored %s\n", e.Path)
			restored++
			done = append(done, e.Path)
			continue
		}

		data, rerr := os.ReadFile(e.Path)
		if rerr == nil && state.Hash(data) != e.Hash {
			fmt.Printf("  kept %s (modified since kit wrote it)\n", e.Path)
			kept++
			continue
		}
		if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
			fmt.Printf("  warning: %s\n", err)
			continue
		}
		fmt.Printf("  removed %s\n", e.Path)
		removed++
		done = append(done, e.Path)
	}

	if *dry {
		fmt.Println("\n(dry-run; nothing changed)")
		return
	}
	for _, p := range done {
		st.Remove(p)
	}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "kit uninstall: "+err.Error())
		os.Exit(1)
	}
	fmt.Printf("\nremoved %d, restored %d, kept %d\n", removed, restored, kept)
}

// resolveOnly merges a --preset, a --components list, and positional ids into
// one de-duplicated component selection. An empty result means "everything".
func resolveOnly(m *manifest.Manifest, preset, components string, positional []string) ([]string, error) {
	var all []string
	if preset != "" {
		ids, err := m.ResolvePreset(preset)
		if err != nil {
			return nil, err
		}
		all = append(all, ids...)
	}
	for _, s := range strings.Split(components, ",") {
		if s = strings.TrimSpace(s); s != "" {
			all = append(all, s)
		}
	}
	all = append(all, positional...)

	seen := make(map[string]bool, len(all))
	out := make([]string, 0, len(all))
	for _, id := range all {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func usage() {
	fmt.Println(`kit — cross-platform terminal kit manager

Usage:
  kit                    launch the TUI
  kit detect             print environment and component status
  kit presets            list the available component presets
  kit themes             list the themes available on this machine
  kit theme [name]       show or set the active theme (zellij + alacritty)
        --dry-run        show what would be written without applying
        -y, --yes        apply without asking for confirmation
        -f, --full       also run the system theme command (e.g. omakub-theme-set)
  kit state              show the files kit manages on this machine
  kit uninstall [flags]  remove (or restore) the files kit wrote
        --dry-run        report without changing anything
        --restore        restore backups instead of deleting
        -y, --yes        do not ask for confirmation
  kit update [flags]     check for and install a newer kit release
        --check          only report whether an update is available
        -y, --yes        update without asking
        --force          reinstall even if already current
  kit export [flags] [id...]
                         export the kit as a secret-sanitized bundle
        --out DIR        output directory (default "bundles")
        --dry-run        report captured files and redactions, write nothing
        --preset NAME    component preset (see: kit presets)
        --components CSV comma-separated component ids
  kit install [flags] [id...]
                         print (and optionally run) the install plan
        --apply          execute the plan; without it this is a dry-run
        -i, --interactive  accept each component one by one
        --preset NAME    component preset (see: kit presets)
        --components CSV comma-separated component ids
  kit restore [flags] <bundle.zip>
                         apply a bundle's configs to this machine
        --dry-run        report destinations without writing
        --force          overwrite existing files
        --target T       destination target: linux|wsl|windows
  kit setup [flags]      install tools and write configs in one run
        --apply          execute (default is a dry-run)
        -i, --interactive  guided: accept tools and configs one by one
        --from FILE      bundle zip to restore configs from (default: seeds)
        --force          overwrite existing config files
        --skip-tools     do not install tools
        --skip-configs   do not write config files
        --preset NAME    component preset (see: kit presets)
        --components CSV comma-separated component ids
        --target T       destination target: linux|wsl|windows
  kit version            print version`)
}
