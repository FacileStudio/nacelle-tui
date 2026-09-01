package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// tasksTool lets the model lay a large job out as steps and keep them up to
// date while it works, drawn live above the prompt.
//
// It holds nothing. That is the whole design rather than an accident of a
// small tool: nacelle.Tool.Run is documented as callable from several
// goroutines at once, and it runs on the agent's goroutine while View renders
// on bubbletea's. A tool that reached into the model would be two goroutines
// writing one field with no lock, which the race detector in check.sh finds
// and a user finds as a half-drawn frame. Everything it has to say goes down
// the reports channel and comes back as a message the update loop routes —
// see delegate.go, which solves the same problem the same way.
//
// It is not wired in here. withTasks in tools_main.go appends tasksTool{} to
// the local set, and it has to run after withSubagents: a delegate inherits
// every tool the parent holds when its own tool is built, and there is one
// plan on one screen. This file only has to be safe to call.
type tasksTool struct{}

// Name is what the model calls.
func (tasksTool) Name() string { return "tasks" }

// Description is prompt engineering, not documentation. Every rule in it is
// there because the opposite behaviour has been seen: a plan written for a
// two-step job is noise, a plan with three steps running at once is a status
// display that says nothing, and a step marked done because the model intends
// to finish it is a lie the reader cannot tell from the truth.
func (tasksTool) Description() string {
	return strings.Join([]string{
		"Lay out the work as a list of steps, shown live to the user.",
		"",
		"Use this when a job is big enough that it needs splitting: several",
		"distinct steps, or work the user asked for as a numbered list. Do not",
		"use it for a job of one or two steps, or for something purely",
		"conversational — a plan for trivial work is noise on the screen.",
		"",
		"Two calling modes:",
		"",
		"1. tasks — the full list of steps. Send this to create or replace the",
		"   whole plan. Call once to set up the plan.",
		"",
		"2. step_update — change one step by its 0-based index. Send only the",
		"   fields that changed (status, title, reason). Much lighter than",
		"   sending the whole list every call.",
		"",
		"Exactly one step is in_progress at any moment, never two and never",
		"none while there is work left. Mark a step in_progress before you",
		"start it, and mark it completed the moment it is really finished.",
		"",
		"If a step cannot proceed, mark it blocked or failed with a reason",
		"telling why. Then adjust the plan — add steps, change approach — and",
		"mark the next one in_progress. The list on screen always reflects",
		"reality. Never mark work completed that you have not done.",
	}, "\n")
}

// Schema is the JSON Schema of the tool's input. The status enum is stated
// here as well as checked in Run, because a model that is told the three
// words rarely invents a fourth — the check is what happens when it does.
//
// Two modes: send the full tasks list to set the whole plan, or send a
// step_update with an index to change one step. step_update is lighter for
// status changes and costs fewer tokens per call.
func (tasksTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tasks": map[string]any{
				"type":        "array",
				"description": "The full list of steps, in order. Provide this to set or replace the whole plan.",
				"items":       stepSchema(),
			},
			"step_update": stepUpdateSchema(),
		},
	}
}

// stepSchema is one step's shape, split out of Schema because a schema
// written as one literal is six maps deep and the gate caps nesting at four.
// It is also the only half worth reading: the enum here is the vocabulary the
// model is given.
func stepSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":  map[string]any{"type": "string", "description": "What the step does, in a few words."},
			"status": map[string]any{"type": "string", "enum": []string{statusTodo, statusActive, statusDone, statusBlocked, statusFailed}},
			"reason": map[string]any{"type": "string", "description": "Why the step is blocked or failed. Only needed for those statuses."},
		},
		"required": []string{"title", "status"},
	}
}

// stepUpdateSchema is the single-step update shape, separate from stepSchema
// because it is not an element of an array — the model sends one at a time,
// keyed by the step's 0-based index in the plan.
func stepUpdateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"index":  map[string]any{"type": "integer", "description": "The step's 0-based position in the plan."},
			"status": map[string]any{"type": "string", "enum": []string{statusTodo, statusActive, statusDone, statusBlocked, statusFailed}},
			"title":  map[string]any{"type": "string", "description": "New title for the step, if changing it."},
			"reason": map[string]any{"type": "string", "description": "Why the step is blocked or failed. Only needed for those statuses."},
		},
		"required": []string{"index"},
	}
}

// Run records the plan the model just wrote and reports back what it now says.
//
// The list is a snapshot, never a diff: what arrives here is the plan in full
// and it replaces whatever was on screen. A diff would need this tool to keep
// the last list between calls, which is state, which is a lock, which is the
// thing the doc comment on tasksTool exists to avoid.
//
// When step_update is provided instead of tasks, it is merged with the current
// plan held in currentPlan so the model does not have to repeat the whole list
// for a single status change.
//
// The send is guarded by the context rather than left to block forever. It
// blocks while the update loop is alive, which is the point — a dropped plan
// is a screen that is quietly wrong — but a cancelled run is a loop that may
// never read again, and a tool goroutine parked on a channel nobody drains
// outlives the run that started it.
func (t tasksTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var reported struct {
		Tasks      taskList `json:"tasks,omitempty"`
		StepUpdate *struct {
			Index  int     `json:"index"`
			Status *string `json:"status,omitempty"`
			Title  *string `json:"title,omitempty"`
			Reason *string `json:"reason,omitempty"`
		} `json:"step_update,omitempty"`
	}
	if err := json.Unmarshal(input, &reported); err != nil {
		return "", fmt.Errorf("tasks: input is not the expected object: %w", err)
	}

	var merged taskList
	var ok bool
	switch {
	case len(reported.Tasks) > 0:
		merged = reported.Tasks
	case reported.StepUpdate != nil:
		cur := currentPlan.Load()
		if cur == nil {
			return "", fmt.Errorf("tasks: step_update needs an existing plan but none has been set")
		}
		merged, ok = cur.(taskList)
		if !ok {
			return "", fmt.Errorf("tasks: internal error — plan is not a taskList")
		}
		if len(merged) == 0 {
			return "", fmt.Errorf("tasks: step_update needs an existing plan but none has been set")
		}
		idx := reported.StepUpdate.Index
		if idx < 0 || idx >= len(merged) {
			return "", fmt.Errorf("tasks: step_update index %d is out of range (plan has %d steps)", idx, len(merged))
		}
		if reported.StepUpdate.Status != nil {
			merged[idx].Status = *reported.StepUpdate.Status
		}
		if reported.StepUpdate.Title != nil {
			merged[idx].Title = *reported.StepUpdate.Title
		}
		if reported.StepUpdate.Reason != nil {
			merged[idx].Reason = *reported.StepUpdate.Reason
		}
	default:
		return "", fmt.Errorf("tasks: provide either tasks (full plan) or step_update (single step)")
	}

	if err := validate(merged); err != nil {
		return "", err
	}

	select {
	case reports <- taskUpdate(merged):
		return summarise(merged), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// validate refuses a list the screen could not honestly draw, naming what is
// wrong so the model can fix it on the next call rather than guess.
//
// Two steps in progress is refused; zero is not. "Exactly one" is the rule the
// description asks for, and it is right while there is work left — but a plan
// whose last step has just been completed is a legitimate final snapshot, and
// refusing it would leave the screen showing the last step as still running
// forever. So the check enforces the half that is unambiguously a mistake.
func validate(list taskList) error {
	active := 0
	for _, item := range list {
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("tasks: every step needs a title")
		}
		switch item.Status {
		case statusTodo, statusDone, statusBlocked, statusFailed:
		case statusActive:
			active++
		default:
			return fmt.Errorf("tasks: %q is not a status, use %s, %s, %s, %s or %s", item.Status, statusTodo, statusActive, statusDone, statusBlocked, statusFailed)
		}
	}
	if active > 1 {
		return fmt.Errorf("tasks: %d steps are %s at once, only one may be", active, statusActive)
	}
	return nil
}

// summarise is what the model reads back: the shape of the plan and the step
// it is meant to be on, short enough that repeating it every call costs
// nothing in context.
func summarise(list taskList) string {
	done, running, blocked, failed := 0, "", 0, 0
	for _, item := range list {
		switch item.Status {
		case statusDone:
			done++
		case statusActive:
			running = item.Title
		case statusBlocked:
			blocked++
		case statusFailed:
			failed++
		}
	}
	var extras []string
	if blocked > 0 {
		extras = append(extras, fmt.Sprintf("%d blocked", blocked))
	}
	if failed > 0 {
		extras = append(extras, fmt.Sprintf("%d failed", failed))
	}
	head := fmt.Sprintf("plan recorded: %d steps, %d done", len(list), done)
	if len(extras) > 0 {
		head += ", " + strings.Join(extras, ", ")
	}
	if running == "" {
		return head + ", none in progress"
	}
	return head + fmt.Sprintf(", now on %q", running)
}
