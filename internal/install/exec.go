package install

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/valenciajoel/kit/internal/inventory"
)

// Runner executes a single installation step.
type Runner interface {
	Run(ctx context.Context, s Step) error
}

// ExecRunner runs steps as real subprocesses, inheriting os.Stdin so that
// privileged or interactive installers can prompt.
type ExecRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run implements Runner. When npm is missing but mise is available, it falls
// back to `mise exec -- npm ...`, so npm-based tools work in the same run that
// just installed Node through mise.
func (r ExecRunner) Run(ctx context.Context, s Step) error {
	program, args := s.Program, s.Args
	if _, err := exec.LookPath(program); err != nil && program == "npm" {
		if _, merr := exec.LookPath("mise"); merr == nil {
			program = "mise"
			args = append([]string{"exec", "--", "npm"}, s.Args...)
		}
	}

	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Stdout = orWriter(r.Stdout, os.Stdout)
	cmd.Stderr = orWriter(r.Stderr, os.Stderr)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", program, err)
	}
	return nil
}

// Execute prints the plan when apply is false, and runs it when apply is true.
func Execute(ctx context.Context, p Plan, runner Runner, apply bool, out io.Writer) error {
	for _, s := range p.Skipped {
		fmt.Fprintln(out, Describe(s))
	}

	for _, s := range p.Steps {
		if !apply {
			fmt.Fprintln(out, Describe(s))
			continue
		}
		fmt.Fprintln(out, "→ "+Describe(s))
		if err := runner.Run(ctx, s); err != nil {
			return fmt.Errorf("component %s: %w", s.ComponentID, err)
		}
	}

	if !apply {
		fmt.Fprintln(out, "\n(dry-run; re-run with --apply to execute)")
	}
	return nil
}

// CandidateBinDirs are directories where freshly installed tools commonly land.
func CandidateBinDirs(env inventory.Environment) []string {
	var dirs []string
	if env.Home != "" {
		dirs = append(dirs,
			filepath.Join(env.Home, ".local", "bin"),
			filepath.Join(env.Home, ".local", "share", "mise", "shims"),
			filepath.Join(env.Home, "go", "bin"),
		)
	}
	if env.OS == inventory.Windows {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dirs = append(dirs, filepath.Join(appdata, "npm"))
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "mise", "shims"), filepath.Join(local, "Programs"))
		}
	}
	dirs = append(dirs, "/usr/local/bin")
	return dirs
}

// AugmentPath prepends existing known install locations to this process's PATH,
// so a single setup run can invoke tools it just installed.
func AugmentPath(env inventory.Environment) {
	current := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	seen := make(map[string]bool)
	for _, p := range filepath.SplitList(current) {
		seen[p] = true
	}

	var prepend []string
	for _, dir := range CandidateBinDirs(env) {
		if dir == "" || seen[dir] {
			continue
		}
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			prepend = append(prepend, dir)
			seen[dir] = true
		}
	}
	if len(prepend) == 0 {
		return
	}
	os.Setenv("PATH", strings.Join(prepend, sep)+sep+current)
}

func orWriter(w, fallback io.Writer) io.Writer {
	if w == nil {
		return fallback
	}
	return w
}
