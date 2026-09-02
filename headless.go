package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/FacileStudio/nacelle"
)

// runHeadless runs a single prompt and streams text to stdout.
// The prompt comes from the argument, or from stdin when piped.
// Exit codes: 0 on clean completion, 1 on error.
func runHeadless(prompt string) error {
	agent, cleanup, err := buildHeadlessAgent()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	conv := []nacelle.Message{nacelle.UserText(prompt)}
	for event, err := range agent.Stream(ctx, conv) {
		if err != nil {
			return err
		}
		if event.Kind == nacelle.KindText {
			fmt.Print(event.Text)
		}
	}
	fmt.Println()
	return nil
}

// buildHeadlessAgent assembles the agent the same way the TUI does,
// without approval-gate wiring or banner construction. It returns the
// agent and a cleanup function the caller must defer.
func buildHeadlessAgent() (*nacelle.Agent, func(), error) {
	config, err := settings(fromFlags())
	if err != nil {
		return nil, nil, err
	}

	set, local, err := localTools(config)
	if err != nil {
		return nil, nil, err
	}

	mcp, local, err := mcpTools(config, local)
	if err != nil {
		_ = set.Close()
		return nil, nil, err
	}

	augmentSystem(&config)
	_, approve := buildApprovals(config)

	hooks, _, err := sessionHooks(config)
	if err != nil {
		_ = set.Close()
		_ = mcp.set.Close()
		return nil, nil, err
	}

	agent, _, err := build(config, local, approve, hooks)
	if err != nil {
		_ = set.Close()
		_ = mcp.set.Close()
		return nil, nil, err
	}

	cleanup := func() {
		_ = set.Close()
		_ = mcp.set.Close()
	}
	return agent, cleanup, nil
}

// stdinPrompt reads the first line of stdin when the terminal is not
// interactive, for piped usage: `echo "list files" | nacelle`.
func stdinPrompt() (string, error) {
	data, err := readStdinFirstLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(data), nil
}

// readStdinFirstLine reads up to the first newline from stdin.
func readStdinFirstLine() (string, error) {
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(tmp)
		if n > 0 {
			idx := strings.IndexByte(string(tmp[:n]), '\n')
			if idx >= 0 {
				buf.Write(tmp[:idx])
				return buf.String(), nil
			}
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	if buf.Len() > 0 {
		return buf.String(), nil
	}
	return "", nil
}
