package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

func TestADelegateWithNoApprovalGateMayCall(t *testing.T) {
	policy := delegateApprovals(nil)
	if policy == nil {
		t.Fatal("no policy was built, which the SDK reads as deny-all")
	}
	if !policy(context.Background(), "read_file", json.RawMessage(`{}`)) {
		t.Error("a call was refused in a session that asks about nothing")
	}
}

func TestADelegateStillObeysTheParentsGate(t *testing.T) {
	asked := ""
	gate := nacelle.Approve(func(_ context.Context, name string, _ json.RawMessage) bool {
		asked = name
		return name == "read_file"
	})

	policy := delegateApprovals(gate)
	if !policy(context.Background(), "read_file", json.RawMessage(`{}`)) {
		t.Error("the gate allowed read_file and the delegate was refused anyway")
	}
	if policy(context.Background(), "run_command", json.RawMessage(`{}`)) {
		t.Error("the gate refused run_command and the delegate ran it")
	}
	if asked != "run_command" {
		t.Errorf("the gate was asked about %q, want the delegate's own calls", asked)
	}
}

// The delegate set is the parallel tool alone: subagents defaults on, so the
// mount is active in every ordinary session, and the single subagent tool is
// not wired beside it.
func TestSubagentsMountsOnlyTheParallelTool(t *testing.T) {
	config := settings.Defaults("")
	on := true
	config.Subagents = &on

	tools, err := withSubagents(config, &answeringStub{}, make([]nacelle.Tool, 0), nil)
	if err != nil {
		t.Fatalf("withSubagents: %v", err)
	}
	for _, tool := range tools {
		if tool.Name() != nacelle.ParallelSubAgentToolName {
			t.Errorf("mounted %q, want only the parallel_subagent tool", tool.Name())
		}
	}
	if len(tools) == 0 {
		t.Error("no delegate tool mounted")
	}
}
