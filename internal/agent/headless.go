package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/nacelle-tui/internal/approval"
	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

// runHeadless runs a single prompt and streams text to stdout.
// The prompt comes from the argument, or from stdin when piped.
// Exit codes: 0 on clean completion, 1 on error.
func runHeadless(prompt string) error {
	flags := settings.FromFlags(settings.Defaults(""))
	config, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return err
	}
	_, err = runHeadlessConfig(prompt, config)
	return err
}

// runHeadlessConfig streams one prompt through an agent built from the given
// config and returns the full text. The caller decides what to do with it —
// the -print path drops it, a cron run delivers it.
func runHeadlessConfig(prompt string, config settings.Config) (string, error) {
	agent, cleanup, err := buildHeadlessAgent(config)
	if err != nil {
		return "", err
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var out strings.Builder
	conv := []nacelle.Message{nacelle.UserText(prompt)}
	for event, err := range agent.Stream(ctx, conv) {
		if err != nil {
			return "", err
		}
		if event.Kind == nacelle.KindText {
			if _, err := fmt.Fprint(os.Stdout, event.Text); err != nil {
				return "", err
			}
			out.WriteString(event.Text)
		}
	}
	fmt.Println()
	out.WriteString("\n")
	return out.String(), nil
}

// buildHeadlessAgent assembles the agent the same way the TUI does,
// without approval-gate wiring or banner construction. It returns the
// agent and a cleanup function the caller must defer.
func buildHeadlessAgent(config settings.Config) (*nacelle.Agent, func(), error) {
	set, local, err := localTools(config)
	if err != nil {
		return nil, nil, err
	}

	mcp, local, err := mcpTools(config, local)
	if err != nil {
		return nil, nil, closeOnErr(err, set)
	}

	augmentSystem(&config)
	_, approve := approval.Build(*config.ApproveTools)

	hooks, _, err := settings.SessionHooks(config)
	if err != nil {
		return nil, nil, closeOnErr(err, set, mcp.set)
	}

	get, err := build(config, local, approve, hooks)
	if err != nil {
		return nil, nil, closeOnErr(err, set, mcp.set)
	}

	return get.agent, func() {
		if err := closeAll(set, mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}

// closeOnErr returns the original err if cleanup succeeds, or the cleanup
// error if cleanup fails. The caller should prefer the cleanup error only
// when it wants to surface close failures over the original failure.
func closeOnErr(err error, closers ...any) error {
	if err == nil {
		return nil
	}
	if cerr := closeAll(closers...); cerr != nil {
		return cerr
	}
	return err
}

// closeAll calls Close on every closer it receives. It returns the last
// error returned by a Close() error call, if any.
func closeAll(closers ...any) error {
	var lastErr error
	for _, c := range closers {
		if c == nil {
			continue
		}
		switch v := c.(type) {
		case interface{ Close() error }:
			if err := v.Close(); err != nil {
				lastErr = err
			}
		case interface{ Close() }:
			v.Close()
		}
	}
	return lastErr
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
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
