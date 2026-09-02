package main

import (
	"fmt"
	"strings"
)

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
