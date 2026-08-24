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
		"Each call replaces the whole list. Send every step every time, not",
		"just the ones that changed.",
		"",
		"Exactly one step is in_progress at any moment, never two and never",
		"none while there is work left. Mark a step in_progress before you",
		"start it, and mark it completed the moment it is really finished.",
		"",
		"Only completed means done. If a step is blocked, failed, or waiting",
		"on something, leave it in_progress and add a new step describing what",
		"is in the way. Never mark work completed that you have not done.",
	}, "\n")
}

// Schema is the JSON Schema of the tool's input. The status enum is stated
// here as well as checked in Run, because a model that is told the three
// words rarely invents a fourth — the check is what happens when it does.
func (tasksTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tasks": map[string]any{
				"type":        "array",
				"description": "The full list of steps, in order.",
				"items":       stepSchema(),
			},
		},
		"required": []string{"tasks"},
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
			"status": map[string]any{"type": "string", "enum": []string{statusTodo, statusActive, statusDone}},
		},
		"required": []string{"title", "status"},
	}
}

// Run records the plan the model just wrote and reports back what it now says.
//
// The list is a snapshot, never a diff: what arrives here is the plan in full
// and it replaces whatever was on screen. A diff would need this tool to keep
// the last list between calls, which is state, which is a lock, which is the
// thing the doc comment on tasksTool exists to avoid.
//
// The send is guarded by the context rather than left to block forever. It
// blocks while the update loop is alive, which is the point — a dropped plan
// is a screen that is quietly wrong — but a cancelled run is a loop that may
// never read again, and a tool goroutine parked on a channel nobody drains
// outlives the run that started it.
func (t tasksTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var reported struct {
		Tasks taskList `json:"tasks"`
	}
	if err := json.Unmarshal(input, &reported); err != nil {
		return "", fmt.Errorf("tasks: input is not the expected object: %w", err)
	}
	if err := validate(reported.Tasks); err != nil {
		return "", err
	}

	select {
	case reports <- taskUpdate(reported.Tasks):
		return summarise(reported.Tasks), nil
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
		case statusTodo, statusDone:
		case statusActive:
			active++
		default:
			return fmt.Errorf("tasks: %q is not a status, use %s, %s or %s", item.Status, statusTodo, statusActive, statusDone)
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
	done, running := 0, ""
	for _, item := range list {
		switch item.Status {
		case statusDone:
			done++
		case statusActive:
			running = item.Title
		}
	}
	if running == "" {
		return fmt.Sprintf("plan recorded: %d steps, %d done, none in progress", len(list), done)
	}
	return fmt.Sprintf("plan recorded: %d steps, %d done, now on %q", len(list), done, running)
}
