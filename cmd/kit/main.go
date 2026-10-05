// Command kit is a cross-platform TUI that detects, exports, and installs the
// user's terminal/dev kit (Zellij + Alacritty + nvim + starship + mise + opencode).
package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valenciajoel/kit/internal/inventory"
	"github.com/valenciajoel/kit/internal/manifest"
	"github.com/valenciajoel/kit/internal/target"
	"github.com/valenciajoel/kit/internal/tui"
	"github.com/valenciajoel/kit/kits"
)

const version = "0.1.0"

func main() {
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
		case "version", "--version", "-v":
			fmt.Println("kit " + version)
		case "help", "--help", "-h":
			usage()
		case "export", "install":
			fmt.Printf("kit %s: not implemented yet (planned T7/T8)\n", args[0])
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

func usage() {
	fmt.Println(`kit — cross-platform terminal kit manager

Usage:
  kit            launch the TUI
  kit detect     print environment and component status
  kit export     export the kit as a portable bundle (planned)
  kit install    install the kit on the current target (planned)
  kit version    print version`)
}
