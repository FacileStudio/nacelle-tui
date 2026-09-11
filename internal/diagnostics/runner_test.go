package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func stubFilet(t *testing.T, fn func(context.Context, string) runOutput) {
	t.Helper()
	old := runFilet
	runFilet = fn
	t.Cleanup(func() { runFilet = old })
}

func nextCall(t *testing.T, calls <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-calls:
		return ctx
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a filet call")
		return nil
	}
}

func TestInjectChecksOnlyTheRequestedPath(t *testing.T) {
	scope := t.Name()
	dirty := scope + "/dirty.go"
	clean := scope + "/clean.go"
	var saw []string
	stubFilet(t, func(_ context.Context, s string) runOutput {
		saw = append(saw, s)
		if s == dirty {
			return runOutput{code: 1, stdout: dirty + ":1:1: error: one [r]\n"}
		}
		return runOutput{code: 0}
	})
	if got := Inject(context.Background(), clean); got != cleanLine {
		t.Errorf("Inject(clean.go) = %q, want silence: findings from dirty.go leaked into a clean file's check", got)
	}
	if got := Inject(context.Background(), dirty); !strings.Contains(got, "one") {
		t.Errorf("Inject(dirty.go) = %q, want its own finding", got)
	}
	if len(saw) != 2 || saw[0] != clean || saw[1] != dirty {
		t.Errorf("scopes checked = %v, want exactly [%s %s] with no repo-wide call", saw, clean, dirty)
	}
}

func TestInjectSuppressesRepeatsForOnePath(t *testing.T) {
	scope := t.Name()
	stubFilet(t, func(context.Context, string) runOutput {
		return runOutput{code: 1, stdout: "a.go:1:1: error: one [r]\na.go:2:2: error: two [r]\n"}
	})
	if got := Inject(context.Background(), scope); !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("first Inject = %q, want both findings", got)
	}
	if got := Inject(context.Background(), scope); got != noNewLine {
		t.Fatalf("repeat Inject = %q, want %q", got, noNewLine)
	}
	stubFilet(t, func(context.Context, string) runOutput {
		return runOutput{code: 1, stdout: "a.go:1:1: error: one [r]\na.go:2:2: error: two [r]\na.go:3:3: error: three [r]\n"}
	})
	if got := Inject(context.Background(), scope); got != "a.go:3:3: three" {
		t.Fatalf("Inject after a new finding = %q, want only the new one", got)
	}
}

func TestInjectTimesOutAndWarmsOncePerPath(t *testing.T) {
	scope := t.Name()
	calls := make(chan context.Context, 8)
	stubFilet(t, func(ctx context.Context, _ string) runOutput {
		select {
		case calls <- ctx:
		default:
		}
		return runOutput{err: context.DeadlineExceeded}
	})
	if got := Inject(context.Background(), scope); got != "" {
		t.Fatalf("Inject = %q, want empty", got)
	}
	if ctx := nextCall(t, calls); ctx == context.Background() {
		t.Fatal("the first call should carry the run deadline, not the background")
	}
	if warm := nextCall(t, calls); warm != context.Background() {
		t.Fatalf("warm context = %v, want background", warm)
	}
	if got := Inject(context.Background(), scope); got != "" {
		t.Fatalf("second Inject = %q, want empty", got)
	}
	nextCall(t, calls)
	select {
	case ctx := <-calls:
		t.Fatalf("warm ran twice for %s: %v", scope, ctx)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunReportsTimeoutAsAnError(t *testing.T) {
	stubFilet(t, func(context.Context, string) runOutput {
		return runOutput{err: context.DeadlineExceeded}
	})
	got, err := Run(context.Background(), t.Name(), false)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if got != "" {
		t.Fatalf("Run = %q, want empty", got)
	}
}

func TestRunChecksTheSessionRootForRepo(t *testing.T) {
	var scope string
	stubFilet(t, func(_ context.Context, s string) runOutput {
		scope = s
		return runOutput{code: 1, stdout: "b.go:1:1: error: boom [r]\nc.go:2:2: error: pow [r]"}
	})
	got, err := Run(context.Background(), "ignored.go", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scope != "." {
		t.Fatalf("scope = %q, want the session root", scope)
	}
	if want := "b.go:1:1: boom\nc.go:2:2: pow"; got != want {
		t.Fatalf("Run = %q, want %q", got, want)
	}
}

func TestRunTreatsABrokenCheckerAsCleanSilence(t *testing.T) {
	stubFilet(t, func(context.Context, string) runOutput { return runOutput{code: 7} })
	got, err := Run(context.Background(), t.Name(), false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != cleanLine {
		t.Fatalf("Run = %q, want %q", got, cleanLine)
	}
}
