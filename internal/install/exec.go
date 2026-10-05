package install

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// Run implements Runner.
func (r ExecRunner) Run(ctx context.Context, s Step) error {
	cmd := exec.CommandContext(ctx, s.Program, s.Args...)
	cmd.Stdout = orWriter(r.Stdout, os.Stdout)
	cmd.Stderr = orWriter(r.Stderr, os.Stderr)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", s.Program, err)
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

func orWriter(w, fallback io.Writer) io.Writer {
	if w == nil {
		return fallback
	}
	return w
}
