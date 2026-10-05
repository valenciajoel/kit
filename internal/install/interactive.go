package install

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// Prompter reads yes/no answers from a shared scanner, so a whole interactive
// session (tools, then configs) can reuse one input stream without losing
// buffered input.
type Prompter struct {
	scanner *bufio.Scanner
	out     io.Writer
}

// NewPrompter builds a Prompter reading from in and writing prompts to out.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{scanner: bufio.NewScanner(in), out: out}
}

// Yes prints prompt and returns true when the answer is affirmative. Anything
// else, including EOF, means no.
func (p *Prompter) Yes(prompt string) bool {
	fmt.Fprint(p.out, prompt)
	if !p.scanner.Scan() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(p.scanner.Text())) {
	case "y", "yes", "s", "si", "sí":
		return true
	default:
		return false
	}
}

// Println writes a line to the prompt output.
func (p *Prompter) Println(s string) { fmt.Fprintln(p.out, s) }

// RunInteractive walks the plan, asking the user to accept each component
// before it is installed.
func RunInteractive(ctx context.Context, plan Plan, runner Runner, prompt *Prompter) error {
	for _, s := range plan.Skipped {
		prompt.Println(Describe(s))
	}

	installed, skipped := 0, 0
	for _, s := range plan.Steps {
		prompt.Println("")
		prompt.Println(Describe(s))
		if !prompt.Yes("  install? [y/N] ") {
			prompt.Println("  skipped")
			skipped++
			continue
		}
		if err := runner.Run(ctx, s); err != nil {
			return fmt.Errorf("component %s: %w", s.ComponentID, err)
		}
		installed++
	}

	prompt.Println(fmt.Sprintf("\ninstalled %d, skipped %d", installed, skipped))
	return nil
}
