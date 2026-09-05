package agent

import (
	"context"
	"encoding/json"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
	"github.com/FacileStudio/nacelle-tui/internal/tui"
)

// withSubagents mounts the delegation tool when the settings ask for one. It
// exists so build stays a readable sequence of wiring rather than growing a
// branch per optional tool: the delegate shares the parent's wrapped backend,
// system prompt, tools and iteration ceiling, and reports its spend to the
// session the same way the parent's own turns do.
func withSubagents(config settings.Config, backend nacelle.Backend, local []nacelle.Tool, approve nacelle.Approve) ([]nacelle.Tool, error) {
	if !*config.Subagents {
		return local, nil
	}
	sub, err := nacelle.NewSubAgentTool(nacelle.Config{
		Backend:       backend,
		System:        config.System,
		Tools:         local,
		MaxIterations: *config.MaxIterations,
	}, nacelle.SubAgentOptions{
		Approve: delegateApprovals(approve),
		Usage:   tui.DelegateUsage,
	})
	if err != nil {
		return nil, err
	}
	return append(local, sub), nil
}

// delegateApprovals is the policy the nested run answers to. It has to be
// stated, because the SDK's default for a nil SubAgentOptions.Approve is
// deny-all: leaving it unset hands the delegate the parent's whole tool set
// and then refuses every call it makes, which is a tool whose description
// promises wide searches and log dumps and which can do neither.
func delegateApprovals(approve nacelle.Approve) nacelle.Approve {
	if approve != nil {
		return approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}
