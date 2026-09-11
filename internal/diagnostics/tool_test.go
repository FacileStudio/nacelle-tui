package diagnostics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestToolExposesTheDiagnosticsSchema(t *testing.T) {
	built := Tool()
	if built.Name() != "diagnostics" {
		t.Fatalf("name = %q, want diagnostics", built.Name())
	}
	if readOnly, ok := built.(nacelle.ReadOnlyTool); !ok || !readOnly.IsReadOnly() {
		t.Fatal("diagnostics must declare itself read only")
	}
	schema, err := json.Marshal(built.Schema())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"path"`, `"repo"`, `"string"`, `"boolean"`, "session root"} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("schema misses %q: %s", want, schema)
		}
	}
	description := built.Description()
	for _, want := range []string{"file:line:col", "errors only", "empty", "clean"} {
		if !strings.Contains(description, want) {
			t.Fatalf("description misses %q: %s", want, description)
		}
	}
}

func TestToolDecodesInputAndRuns(t *testing.T) {
	var scope string
	stubFilet(t, func(_ context.Context, s string) runOutput {
		scope = s
		return runOutput{code: 1, stdout: "a.go:1:1: error: boom [r]"}
	})
	built := Tool()
	got, err := built.Run(context.Background(), json.RawMessage(`{"path":"a.go"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scope != "a.go" || got != "a.go:1:1: boom" {
		t.Fatalf("scope %q result %q, want a.go with the finding", scope, got)
	}
	got, err = built.Run(context.Background(), json.RawMessage(`{"repo":true}`))
	if err != nil {
		t.Fatalf("Run repo: %v", err)
	}
	if scope != "." {
		t.Fatalf("repo scope = %q, want the session root", scope)
	}
	if got != "a.go:1:1: boom" {
		t.Fatalf("repo result = %q", got)
	}
}

func TestToolRejectsMalformedInputBeforeItReachesFilet(t *testing.T) {
	called := false
	stubFilet(t, func(context.Context, string) runOutput {
		called = true
		return runOutput{}
	})
	if _, err := Tool().Run(context.Background(), json.RawMessage(`{"path":`)); err == nil {
		t.Fatal("want a decode error from the SDK")
	}
	if called {
		t.Fatal("malformed input must not reach the filet runner")
	}
}
