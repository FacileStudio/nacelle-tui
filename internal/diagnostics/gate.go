// Gate execution for project-configured gate chains: one deterministic check
// run against a scope with its own timeout budget, exec indirection kept on a
// package var so tests can stand in fake gates.
package diagnostics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultGateTimeout = 8 * time.Second

// Gate is one entry of a configured gate chain. Scope selects what the gate
// runs against: "file" gates run after each edit with the edited path as
// their final argument, "repo" gates run on demand with no extra argument.
// An empty scope means "file".
type Gate struct {
	Name        string
	Cmd         []string
	Scope       string
	TimeoutSecs int
}

func (g Gate) fileScoped() bool {
	return g.Scope != "repo"
}

func (g Gate) timeout() time.Duration {
	if g.TimeoutSecs > 0 {
		return time.Duration(g.TimeoutSecs) * time.Second
	}
	return defaultGateTimeout
}

type gateOutput struct {
	stdout string
	stderr string
	code   int
	err    error
}

var runGate = execGate

func execFilet(ctx context.Context, scope string) gateOutput {
	cmd := exec.CommandContext(ctx, "filet", "check", scope)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exit.ExitCode()
		}
	}
	return gateOutput{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}

func execGate(ctx context.Context, g Gate, scope string) gateOutput {
	runCtx, cancel := context.WithTimeout(ctx, g.timeout())
	defer cancel()
	args := append([]string{}, g.Cmd[1:]...)
	if g.fileScoped() {
		args = append(args, scope)
	}
	cmd := exec.CommandContext(runCtx, g.Cmd[0], args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exit.ExitCode()
		}
	}
	return gateOutput{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}

func gateOutcome(g Gate, out gateOutput) outcome {
	if errors.Is(out.err, context.DeadlineExceeded) {
		return outcome{kind: kindTimedOut}
	}
	switch out.code {
	case 0:
		return outcome{kind: kindClean}
	case 1:
		if fs := parse(out.stdout); len(fs) > 0 {
			return outcome{kind: kindFindings, findings: fs}
		}
		return outcome{kind: kindFindings, raw: rawText(out)}
	default:
		text := rawText(out)
		if text == "" {
			text = out.err.Error()
		}
		return outcome{kind: kindFindings, raw: fmt.Sprintf("%s failed (exit %d): %s", g.Name, out.code, text)}
	}
}

func rawText(out gateOutput) string {
	text := strings.TrimSpace(out.stdout)
	if extra := strings.TrimSpace(out.stderr); extra != "" {
		text = strings.TrimSpace(text + "\n" + extra)
	}
	return text
}
