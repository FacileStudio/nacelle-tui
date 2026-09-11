package agent

import (
	"context"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestWithDiagnosticsHookAddsAfterToolCall(t *testing.T) {
	hooks := withDiagnosticsHook(nil)
	if len(hooks[nacelle.AfterToolCall]) != 1 {
		t.Fatalf("hooks = %d, want the one diagnostics hook", len(hooks[nacelle.AfterToolCall]))
	}
}

func TestWithDiagnosticsHookCopiesCallerMap(t *testing.T) {
	src := map[nacelle.HookPoint][]nacelle.Hook{
		nacelle.BeforeToolCall: {func(context.Context, nacelle.HookEvent) nacelle.HookResult { return nacelle.HookResult{} }},
	}
	hooks := withDiagnosticsHook(src)
	if len(src[nacelle.AfterToolCall]) != 0 {
		t.Error("the caller's map was mutated")
	}
	if len(hooks[nacelle.BeforeToolCall]) != 1 {
		t.Error("existing hooks were dropped")
	}
}

func TestDiagnosticsHookFilters(t *testing.T) {
	hook := withDiagnosticsHook(nil)[nacelle.AfterToolCall][0]
	ctx := context.Background()

	if res := hook(ctx, nacelle.HookEvent{Tool: "read_file", Input: `{"path":"main.go"}`}); res.Inject != "" {
		t.Errorf("read_file triggered injection: %q", res.Inject)
	}
	if res := hook(ctx, nacelle.HookEvent{Tool: "edit_file", Err: context.Canceled}); res.Inject != "" {
		t.Errorf("a failed edit triggered injection: %q", res.Inject)
	}
	if res := hook(ctx, nacelle.HookEvent{Tool: "edit_file", Input: `{oops`}); res.Inject != "" {
		t.Errorf("malformed input triggered injection: %q", res.Inject)
	}
	if res := hook(ctx, nacelle.HookEvent{Tool: "edit_file", Input: `{"path":""}`}); res.Inject != "" {
		t.Errorf("empty path triggered injection: %q", res.Inject)
	}
}
