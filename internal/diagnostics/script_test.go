package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withFilet(t *testing.T, body, scope string) string {
	t.Helper()
	bin := t.TempDir()
	script := filepath.Join(bin, "filet")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return scope
}

func TestInjectMapsExitCodesFromARealBinary(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"clean", "exit 0", cleanLine},
		{"findings", "echo 'a.go:3:5: error: undefined: foo [go.typecheck]'\nexit 1", "a.go:3:5: undefined: foo"},
		{"other", "exit 7", ""},
		{"bad path", "exit 2", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope := withFilet(t, tc.body, tc.name+".go")
			if got := Inject(context.Background(), scope); got != tc.want {
				t.Fatalf("Inject = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInjectStaysSilentWithoutFiletOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := Inject(context.Background(), "gone.go"); got != "" {
		t.Fatalf("Inject = %q, want empty", got)
	}
}

func TestInjectStaysSilentWithoutAConfig(t *testing.T) {
	body := "echo 'filet: no filet.yml under x.go, checking against the defaults' >&2\n" +
		"echo 'x.go:1:1: error: boom [go.boom]'\nexit 1"
	scope := withFilet(t, body, "noconf.go")
	if got := Inject(context.Background(), scope); got != "" {
		t.Fatalf("Inject = %q, want empty", got)
	}
}

func TestInjectTimesOutAgainstASlowBinary(t *testing.T) {
	scope := withFilet(t, "exec /bin/sleep 3", "slow.go")
	start := time.Now()
	if got := Inject(context.Background(), scope); got != "" {
		t.Fatalf("Inject = %q, want empty", got)
	}
	if elapsed := time.Since(start); elapsed < runTimeout {
		t.Fatalf("Inject returned after %s, want the %s kill first", elapsed, runTimeout)
	}
}
