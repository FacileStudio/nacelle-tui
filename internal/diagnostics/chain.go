// The gate chain: project-configured deterministic checks dispatched at two
// entry points. File-scoped gates run on the post-edit injection path; repo
// gates run only when the pull tool asks for the whole tree. No configured
// chain keeps every caller on the built-in filet path.

package diagnostics

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
)

// gateChain is the surface an installed chain provides; a nil chain is the
// filet-only path every existing caller gets.
type gateChain interface {
	InjectFile(ctx context.Context, path string) string
	Run(ctx context.Context, path string, repo bool) (string, error)
}

var installed atomic.Value

// UseChain installs the chain the entry points consult; nil restores the
// filet-only path.
func UseChain(c gateChain) {
	installed.Store(chainHolder{c})
}

type chainHolder struct {
	chain gateChain
}

func current() gateChain {
	if v, ok := installed.Load().(chainHolder); ok {
		return v.chain
	}
	return nil
}

// NewChain builds a chain from configured gates. Gates run in order, first
// failure stops the run, and a gate whose output parses as compiler-style
// findings renders like filet's; anything else passes through raw, capped.
func NewChain(gates []Gate) *runner {
	return &runner{gates: gates}
}

type runner struct {
	gates []Gate
}

// InjectFile runs the file-scoped gates against one edited path and returns
// the text appended to the edit tool result. Gates that fail oddly stay
// silent so a broken gate never denies the edit it is reporting on.
func (r *runner) InjectFile(ctx context.Context, path string) string {
	for _, g := range r.gates {
		if !g.fileScoped() {
			continue
		}
		out := gateOutcome(g, runGate(ctx, g, path))
		if text, done := chainText(g, out); done {
			return text
		}
	}
	return cleanLine
}

// Run sweeps the chain: file gates against the scope, repo gates against the
// session root, first failure stops.
func (r *runner) Run(ctx context.Context, path string, repo bool) (string, error) {
	scope := scopeOf(path, repo)
	for _, g := range r.gates {
		local := scope
		if !g.fileScoped() {
			local = "."
		}
		out := gateOutcome(g, runGate(ctx, g, local))
		if text, done := chainText(g, out); done {
			if out.kind == kindTimedOut {
				return "", fmt.Errorf("%s: timed out after %s: %w", g.Name, g.timeout(), context.DeadlineExceeded)
			}
			return text, nil
		}
	}
	return cleanLine, nil
}

func chainText(g Gate, out outcome) (string, bool) {
	switch out.kind {
	case kindClean:
		return "", false
	case kindTimedOut:
		return fmt.Sprintf("%s: timed out after %s", g.Name, g.timeout()), true
	case kindFindings:
		if len(out.findings) > 0 {
			return render(out.findings), true
		}
		return renderRaw(g.Name, out.raw), true
	}
	return "", false
}

// renderRaw passes a gate's unparsable output through verbatim, capped at the
// same finding-count and byte budgets as the compiler-style renderer, with
// the overflow spilled to a file the pointer line names.
func renderRaw(gate, text string) string {
	if text == "" {
		return fmt.Sprintf("%s: failed with exit code 1", gate)
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxFindings || joined(lines) > maxTextBytes {
		head := fit(lines[:min(len(lines), maxFindings-1)], 0)
		name, err := spillFile(text)
		if err != nil {
			return strings.Join(head, "\n")
		}
		pointer := fmt.Sprintf("%s: %d lines of output; full text: %s", gate, len(lines), name)
		return strings.Join(append(head, pointer), "\n")
	}
	return strings.Join(lines, "\n")
}
