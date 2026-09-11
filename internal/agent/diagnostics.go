package agent

import (
	"context"
	"encoding/json"
	"maps"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/diagnostics"
)

// The diagnostics loop: filet findings injected after every edit, plus the
// pull tool. Built in, not a settings hook, so the kill switch is the single
// tools.diagnostics toggle and the injection lands even with no hooks file.

// editTools are the tools whose result a diagnostics injection rides on.
var editTools = map[string]bool{"edit_file": true, "write_file": true}

// editInput is the slice of the model's raw tool arguments the injection
// needs. The input arrives undecoded, and malformed JSON is silence: a hook
// error would deny the edit it is reporting on.
type editInput struct {
	Path string `json:"path"`
}

// withDiagnosticsHook returns the hooks map with the post-edit injection
// appended, copying on write so the caller's map is untouched.
func withDiagnosticsHook(hooks map[nacelle.HookPoint][]nacelle.Hook) map[nacelle.HookPoint][]nacelle.Hook {
	diag := func(ctx context.Context, ev nacelle.HookEvent) nacelle.HookResult {
		if !editTools[ev.Tool] || ev.Err != nil {
			return nacelle.HookResult{}
		}
		var in editInput
		if err := json.Unmarshal([]byte(ev.Input), &in); err != nil || in.Path == "" {
			return nacelle.HookResult{}
		}
		return nacelle.HookResult{Inject: diagnostics.Inject(ctx, in.Path)}
	}
	out := maps.Clone(hooks)
	if out == nil {
		out = map[nacelle.HookPoint][]nacelle.Hook{}
	}
	out[nacelle.AfterToolCall] = append(out[nacelle.AfterToolCall], diag)
	return out
}
