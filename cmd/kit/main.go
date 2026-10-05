// Command kit is a cross-platform TUI that detects, exports, and installs the
// user's terminal/dev kit (Zellij + Alacritty + nvim + starship + mise + opencode).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/bundle"
	"github.com/valenciajoel/kit/internal/install"
	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/restore"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/internal/tui"
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

	program := tea.NewProgram(tui.New(m, env, tgt), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kit: "+err.Error())
		os.Exit(1)
	}
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
	_ = fs.Parse(args)

	res, err := bundle.Export(bundle.Options{
		Manifest: m,
		Env:      env,
		Target:   tgt,
		OutDir:   *out,
		DryRun:   *dry,
		Only:     fs.Args(),
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
	_ = fs.Parse(args)

	plan := install.BuildPlan(m, tgt, fs.Args())
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

	res, err := restore.Restore(restore.Options{
		BundlePath: fs.Arg(0),
		Manifest:   m,
		Env:        env,
		Target:     rt,
		DryRun:     *dry,
		Force:      *force,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kit restore: "+err.Error())
		os.Exit(1)
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
	_ = fs.Parse(args)

	rt := target.Target(*targetName)
	if !rt.Valid() {
		fmt.Fprintf(os.Stderr, "kit setup: invalid target %q\n", *targetName)
		os.Exit(2)
	}

	runner := install.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr}

	if *interactive {
		fmt.Printf("kit setup (target=%s, interactive)\n", rt)
		prompt := install.NewPrompter(os.Stdin, os.Stdout)
		install.AugmentPath(env)

		if !*skipTools {
			fmt.Println("\n== tools ==")
			plan := install.BuildPlan(m, rt, nil)
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
				})
				if err != nil {
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
		plan := install.BuildPlan(m, rt, nil)
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
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "kit setup: "+err.Error())
			os.Exit(1)
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
  kit export [flags] [id...]
                         export the kit as a secret-sanitized bundle
        --out DIR        output directory (default "bundles")
        --dry-run        report captured files and redactions, write nothing
  kit install [flags] [id...]
                         print (and optionally run) the install plan
        --apply          execute the plan; without it this is a dry-run
        -i, --interactive  accept each component one by one
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
        --target T       destination target: linux|wsl|windows
  kit version            print version`)
}
