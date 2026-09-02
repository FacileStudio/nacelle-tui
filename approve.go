package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// approvalDecision is the answer to one pending approvalRequest.
type approvalDecision int

const (
	denied approvalDecision = iota
	allowedOnce
	allowedForSession
)

// approvalRequest is one tool call waiting on a human decision. It is sent
// into the running program from whichever goroutine is asking — inside
// RunTool, on whatever goroutine a backend runs tool calls on — which then
// blocks on decision until Update answers it.
type approvalRequest struct {
	name     string
	input    json.RawMessage
	decision chan<- approvalDecision
}

// approvals is the nacelle.Approve function this client hands to
// nacelle.Config when -approve-tools is on, plus the state it needs shared
// across two goroutines: the one asking (inside RunTool) and the update
// loop answering. asking serializes prompts to one at a time — a backend
// can run several tool calls concurrently, and this model has exactly one
// place to show a question.
//
// send is a func rather than holding *tea.Program directly so ask is
// testable without a live terminal: a test supplies its own send and
// answers the request itself. main.go wires it to a real Program's Send
// once the Program exists, which is necessarily after this struct is
// constructed — building the Program needs the model, which needs the
// Agent, which needs this Approve func in hand first. That ordering is safe
// because nothing can call ask before the interactive loop starts and a
// tool call actually happens, by which point send has always been set.
type approvals struct {
	asking sync.Mutex

	mu      sync.Mutex
	allowed map[string]bool

	send func(tea.Msg)
}

func newApprovals() *approvals {
	return &approvals{allowed: map[string]bool{}}
}

// ask is the nacelle.Approve function itself. It blocks the calling
// goroutine until a decision arrives or ctx is cancelled — the same ctrl+c
// that already unblocks a wedged tool unblocks a wedged approval too, since
// both wait on the same run's context. Without that escape hatch, a person
// who walks away mid-question would hang the whole client with no way out
// but kill -9.
//
// Input whose keys repeat is refused here without ever being shown, before
// the session allow-list is even consulted. The status line renders the input
// bytes and the tool runs the decoded value, and a duplicate key is exactly
// the case where those two disagree — {"command":"ls","command":"rm -rf /"}
// reads as ls and executes rm -rf /. The alternative was to prompt and say
// the input is ambiguous, but a prompt still has a y key on it, and offering
// someone a yes for a call nobody can render is not a safer prompt, it is the
// same lie with a warning label. Refusing costs the model one tool call and
// tells it so — nacelle reports a declined call as Refused with an error the
// model reads, which is machinery this client already has — where approving
// the wrong half of an ambiguous call costs whatever the second value did.
//
// The check runs ahead of isAllowed because allow-for-this-session was
// granted against a legible call. It is permission for a tool, not a standing
// waiver on inputs nobody has been able to read since.
//
// The plan tool is exempt and never prompts. It has no effect outside this
// process: it validates a list of steps and hands it to the update loop to be
// drawn, and the whole of what it did is then on screen anyway, which is a
// better report than any prompt would be. The model is told to re-send the
// plan every time a step changes, so asking about it costs roughly one
// keypress per step of every plan — enough to make -approve-tools not worth
// turning on, which is the failure mode where a safety feature stays off.
//
// Matching on the name is safe here in a way it would not be in general. Every
// tool in this process has a fixed name, and a bridged MCP tool is always
// composed as server_remote by the SDK (mcp/client/bridge.go, compose), so a
// bare "tasks" cannot be anything but ours. The comparison goes through
// tasksTool{}.Name() rather than a literal so that renaming the tool cannot
// quietly leave a dead exemption behind.
//
// isAllowed is checked twice, once before the asking lock and once after:
// the second check is for a call that arrived while waiting for its turn to
// ask, and whose tool was allowed for the session by whichever call was
// asked first.
func (a *approvals) ask(ctx context.Context, name string, input json.RawMessage) bool {
	if _, err := strictObject(input); errors.Is(err, errDuplicateKey) {
		return false
	}

	if name == (tasksTool{}).Name() {
		return true
	}

	if a.isAllowed(name) {
		return true
	}

	a.asking.Lock()
	defer a.asking.Unlock()

	if a.isAllowed(name) {
		return true
	}

	decision := make(chan approvalDecision, 1)
	a.send(approvalRequest{name: name, input: input, decision: decision})

	select {
	case d := <-decision:
		if d == allowedForSession {
			a.allow(name)
		}
		return d != denied
	case <-ctx.Done():
		return false
	}
}

func (a *approvals) isAllowed(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allowed[name]
}

func (a *approvals) allow(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowed[name] = true
}

// decide answers the pending approval with whichever of y/a/n resolved it,
// and swallows every other key so a stray press cannot leak into the prompt
// or scroll the transcript while a question is open.
//
// run.pending is cleared here directly, on the UI goroutine, rather than
// left for ask's own ctx.Done() branch to unblock: that branch only fires on
// cancellation, and answering y/a/n is not a cancellation — nothing else
// would ever clear pending on the ordinary path, and the status line would
// keep asking a question that had already been answered.
func (m *model) decide(press tea.KeyPressMsg) tea.Cmd {
	var decision approvalDecision
	switch press.String() {
	case "y":
		decision = allowedOnce
	case "a":
		decision = allowedForSession
	case "n":
		decision = denied
	default:
		return nil
	}

	pending := m.run.pending
	m.run.pending = nil
	pending.decision <- decision
	return nil
}

// accept refuses nothing but ambiguous tool input — the one case where
// encoding/json silently decides for the caller — and lets everything else
// through. It is the default approval function, active whether or not
// -approve-tools was asked for.
func (a *approvals) accept(ctx context.Context, name string, input json.RawMessage) bool {
	if _, err := strictObject(input); errors.Is(err, errDuplicateKey) {
		return false
	}
	return true
}

// buildApprovals constructs the approval gate and returns the approval
// function the core will call on every tool invocation.
//
// When -approve-tools is on the gate is returned so it can be wired to the
// running program — the approval function prompts the human for every call.
// When it is off the gate is nil and the approval function only rejects
// ambiguous input; the tool runs unasked as it always has for everyone
// who never turns the gate on.
func buildApprovals(config Config) (*approvals, nacelle.Approve) {
	gate := newApprovals()
	if *config.ApproveTools {
		return gate, gate.ask
	}
	return nil, gate.accept
}

// wireApprovals connects a non-nil gate to the running program's Send, once
// the program exists to connect it to. It cannot happen any earlier:
// building the program needed the model, which needed the Agent, which
// needed approve in hand first. That ordering is safe because nothing can
// call ask before .Run() starts the interactive loop and a tool call
// actually happens, by which point this has always run.
func wireApprovals(gate *approvals, program *tea.Program) {
	if gate != nil {
		gate.send = program.Send
	}
}
