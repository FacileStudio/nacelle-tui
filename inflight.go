package main

import (
	"context"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// answerStream holds the text buffers produced during a run. Embedded in
// inflight so every field still reads as m.run.answer, m.run.fullAnswer, etc.
type answerStream struct {
	answer        strings.Builder
	fullAnswer    strings.Builder
	committedLen  int
	reasoning     strings.Builder
	reasoningFull strings.Builder
}

// runControl holds the run's coordination state. Embedded in inflight so every
// field still reads as m.run.results, m.run.cancel, etc.
type runControl struct {
	results <-chan result
	cancel  context.CancelFunc
	usage   nacelle.Usage
	stop    nacelle.Stop
	busy    bool
	pending *approvalRequest
	queued  []string
}

// failureCollapse tracks consecutive identical tool failures so they
// render as one line with a count rather than as N identical two-line
// blocks. Cleared at the same edges as the rest of a run's state: flush
// in stranded and settle, reset in clearGroups.
type failureCollapse struct {
	toolLine string
	name     string
	err      string
	duration time.Duration
	count    int
}

// inflight is the one run this client allows at a time: how to hear from it,
// how to abandon it, what it has produced, and what it has cost.
type inflight struct {
	runControl
	answerStream
	editState
	clock
	turn
	groups     []toolGroup
	groupIndex map[string]int
	failures   failureCollapse
}

// clock is when this run started and when esc was first pressed this one.
type clock struct {
	began       time.Time
	interrupted time.Time
}

// turn is the assistant turn being built for the conversation: the tools it
// asked for, and the results collected to answer them.
type turn struct {
	asked    []nacelle.Part
	answered []nacelle.Part
	reported bool
}

// editState is what a run tracks per tool call, plus what drawing a diff for
// its file edits needs: the directory the file tools work in, whether diffs
// were asked for at all, and the before/after of every editing call in
// flight, keyed by call id. It lives on the run rather than the model because
// a run's edits are not a property of the client.
type editState struct {
	root  string
	diffs bool
	edits map[string]editChange
}

// beginTool turns a call into a group row. A new call either extends the
// previous group — same tool kind, and the previous one has not returned yet —
// or starts a fresh row. Grouping by kind aggregates consecutive calls of the
// same category (read, write, network, delegate) even when the specific tool
// or input differs, showing a combined line like "⏺ 4 commands · cmd1 · cmd2 · …".
func (r *inflight) appendToLastGroup(ev nacelle.ToolEvent) bool {
	if len(r.groups) == 0 {
		return false
	}
	g := &r.groups[len(r.groups)-1]
	if !g.end.IsZero() || toolKind(ev.Name) != toolKind(g.tool.Name) {
		return false
	}
	g.count++
	g.callNames = append(g.callNames, primaryArg(ev.Input))
	if ev.ID != "" {
		r.groupIndex[ev.ID] = len(r.groups) - 1
	}
	return true
}

func (r *inflight) beginTool(ev nacelle.ToolEvent, groupTools bool) {
	if r.groups == nil {
		r.groups = make([]toolGroup, 0, 4)
	}
	if r.groupIndex == nil {
		r.groupIndex = make(map[string]int, 4)
	}
	if groupTools && r.appendToLastGroup(ev) {
		return
	}
	g := toolGroup{
		name:      ev.Name,
		input:     ev.Input,
		count:     1,
		tool:      ev,
		start:     time.Now(),
		callNames: []string{primaryArg(ev.Input)},
	}
	r.groups = append(r.groups, g)
	if ev.ID != "" {
		r.groupIndex[ev.ID] = len(r.groups) - 1
	}
}

// finishTool marks a group's row as returned, with the outcome and the
// duration. It looks the group up by ID rather than by position, because a
// result can arrive for a call that was not the most recent one — the model
// fires several reads in parallel and they land back in whatever order the
// filesystem gives.
func (r *inflight) finishTool(ev nacelle.ToolEvent) {
	if ev.ID == "" {
		for i := len(r.groups) - 1; i >= 0; i-- {
			if r.groups[i].end.IsZero() {
				r.groups[i].finishCall(ev)
				return
			}
		}
		return
	}
	if i, ok := r.groupIndex[ev.ID]; ok && i < len(r.groups) {
		r.groups[i].finishCall(ev)
	}
}

// isGroupFailed reports whether any call in a group failed.
func (r *inflight) isGroupFailed(id string) bool {
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return false
	}
	return r.groups[i].failed
}

// isGroup reports whether a tool ID belongs to a multi-call group.
func (r *inflight) isGroup(id string) bool {
	if id == "" {
		return false
	}
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return false
	}
	return r.groups[i].count > 1
}

// heldLine returns the line a finished call will print, and whether the call
// had one. It looks the group up by ID — the same lookup finishTool uses — so
// a result that arrives for a call the model did not name still finds the one
// unfinished row there.
func (r *inflight) heldLine(id string, width int) (string, bool) {
	if id == "" {
		return "", false
	}
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return "", false
	}
	return r.groups[i].groupLine(width), true
}

// isGroupComplete reports whether all calls in a group have finished.
// Returns true for non-grouped tools (count == 1) and for groups where
// finishedCount has caught up to count.
func (r *inflight) isGroupComplete(id string) bool {
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return true
	}
	g := &r.groups[i]
	if g.count <= 1 {
		return true
	}
	return g.finishedCount >= g.count
}

// clearGroups drops the run's tool rows. It is called from settle and from
// stranded, the same two places that empty running and edits, so the three
// stay in lockstep and a run never inherits another run's tool rows.
func (r *inflight) clearGroups() {
	r.groups = nil
	r.groupIndex = nil
}
