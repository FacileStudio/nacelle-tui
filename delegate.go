package main

import (
	"context"
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// delegations carries what every delegated run spends, from the tool's
// goroutine to the update loop. It is buffered so a burst of nested turns
// bridging two frames still lands, and the send in main.go blocks when it
// is full: a dropped spend is a total that is quietly wrong forever, where
// a blocked callback is one buffered turn arriving a frame late.
var delegations = make(chan nacelle.Usage, 64)

// spentDelegation says the session's totals grew by work the parent never
// streamed — a sub-agent's turns arrive here rather than as KindTurn events,
// because the parent's stream sees only the one tool call.
type spentDelegation struct {
	usage nacelle.Usage
}

// watchDelegations waits for one delegated spend and reports it as a message.
// Every handling re-arms it, so it sits listening for the whole session the
// same way waitFor sits on the run's own results.
func watchDelegations() tea.Cmd {
	return func() tea.Msg {
		usage := <-delegations
		return spentDelegation{usage: usage}
	}
}

// record folds one delegated spend into the run in flight. The footer adds
// that to what the session had already spent, so a delegation's cost shows up
// while it runs instead of only on the receipt.
func (m *model) recordDelegation(spent spentDelegation) tea.Cmd {
	m.run.usage = m.run.usage.Add(spent.usage)
	return watchDelegations()
}

// withSubagents mounts the delegation tool when the settings ask for one. It
// exists so build stays a readable sequence of wiring rather than growing a
// branch per optional tool: the delegate shares the parent's wrapped backend,
// system prompt, tools and iteration ceiling, and reports its spend to the
// session the same way the parent's own turns do.
func withSubagents(config Config, backend nacelle.Backend, local []nacelle.Tool, approve nacelle.Approve) ([]nacelle.Tool, error) {
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
		Usage:   func(u nacelle.Usage) { delegations <- u },
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
//
// A nil approve means the parent runs every call unasked, so the delegate
// does too. It is the same process, against the same root, under the same
// -bash setting; there is no power here the parent did not already have, and
// anything stricter is a tool set the model may look at and never use.
//
// A non-nil approve is the parent's own prompt, and it is safe to ask from
// inside a nested run. ask blocks the goroutine that called it and is
// answered by the update loop, which is exactly what it already does for
// every tool call the parent makes, and the asking mutex is released before
// the tool runs, so nothing nests.
//
// The limitation is worth writing down rather than discovering: the prompt
// names the tool and has no way to say a delegate asked for it, and answering
// "allow for the session" widens that tool for the parent too, because the
// allow-list is keyed by name. Same tool, same power, same root — so the
// answer is the same answer either way — but the reader is not told which run
// is asking.
func delegateApprovals(approve nacelle.Approve) nacelle.Approve {
	if approve != nil {
		return approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}
