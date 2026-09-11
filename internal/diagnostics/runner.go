// Package diagnostics surfaces filet's errors to the model: passive injection
// after an edit and an on-demand pull tool, both rendered as compiler-style
// one-liners with attribution dedup and hard caps.
package diagnostics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	runTimeout   = 2 * time.Second
	maxFindings  = 10
	maxTextBytes = 8 * 1024
	noConfig     = "no filet.yml"
	cleanLine    = "filet: ran clean, no diagnostics"
	noNewLine    = "filet: no new diagnostics"
)

type kind int

const (
	kindSilent kind = iota
	kindClean
	kindFindings
	kindTimedOut
)

type outcome struct {
	kind     kind
	findings []finding
}

type runOutput struct {
	stdout string
	stderr string
	code   int
	err    error
}

var runFilet = execFilet

var warming sync.Map

// Inject runs the filet checker on one edited file and returns the text to
// append to the edit tool result: compiler-style file:line:col one-liners,
// errors only, capped at ten findings and eight kilobytes with any overflow
// spilled to a file a pointer line names. Findings already injected for the
// same path in this process are not repeated. The empty string comes back
// when filet is unavailable, when the run exceeds its two second budget, or
// on any other failure; a timed out run starts one background warm-up per
// path so the next call is fast. Inject never returns an error.
func Inject(ctx context.Context, path string) string {
	scope := scopeOf(path, false)
	out := run(ctx, scope)
	switch out.kind {
	case kindFindings:
		fresh := prior.sift(scope, out.findings)
		if len(fresh) == 0 {
			return noNewLine
		}
		return render(fresh)
	case kindClean:
		return cleanLine
	default:
		return ""
	}
}

// Run is the pull side of the package: it checks one path, or the whole
// session root when repo is true, and returns the same compiler-style,
// errors-only, capped rendering for the diagnostics tool. A clean run and an
// unavailable checker both answer with the short clean line, because a
// missing checker is not a reason to stop the model; a timed out run returns
// an error that invites a retry while the background warm-up runs.
func Run(ctx context.Context, path string, repo bool) (string, error) {
	scope := scopeOf(path, repo)
	out := run(ctx, scope)
	switch out.kind {
	case kindFindings:
		prior.learn(scope, out.findings)
		return render(out.findings), nil
	case kindClean:
		prior.learn(scope, nil)
		return cleanLine, nil
	case kindTimedOut:
		return "", fmt.Errorf("filet: timed out after %s; a warm-up is running in the background, try again shortly", runTimeout)
	default:
		return cleanLine, nil
	}
}

func run(ctx context.Context, scope string) outcome {
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	out := runFilet(runCtx, scope)
	if timedOut(runCtx, out) {
		warm(scope)
		return outcome{kind: kindTimedOut}
	}
	if strings.Contains(out.stderr, noConfig) {
		return outcome{kind: kindSilent}
	}
	switch out.code {
	case 0:
		return outcome{kind: kindClean}
	case 1:
		fs := parse(out.stdout)
		if len(fs) == 0 {
			return outcome{kind: kindClean}
		}
		return outcome{kind: kindFindings, findings: fs}
	default:
		return outcome{kind: kindSilent}
	}
}

func timedOut(runCtx context.Context, out runOutput) bool {
	return errors.Is(runCtx.Err(), context.DeadlineExceeded) ||
		errors.Is(out.err, context.DeadlineExceeded)
}

func scopeOf(path string, repo bool) string {
	if repo || path == "" {
		return "."
	}
	return path
}

func warm(scope string) {
	if _, started := warming.LoadOrStore(scope, struct{}{}); started {
		return
	}
	filet := runFilet
	go func() {
		filet(context.Background(), scope)
	}()
}

func execFilet(ctx context.Context, scope string) runOutput {
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
	return runOutput{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}
