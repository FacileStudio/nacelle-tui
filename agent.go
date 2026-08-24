package main

import (
	"fmt"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/anthropic"
	"github.com/FacileStudio/nacelle/openrouter"
)

// Assembling the agent from settings, split out of main.go because that file
// sat on filet's 250-line cap and this is the seam already there: main.go
// decides what the settings are, and this turns them into the thing that
// answers. Nothing here reads a flag or touches the terminal.

// build assembles the agent the settings describe, and hands the backend back
// so the caller can say which one answered. approve is nil unless
// -approve-tools was asked for — see nacelle.Approve's own doc comment for
// why nil, not a rubber-stamp function, is what "off" means here.
//
// The three reasoning settings fold into one nacelle.Thinking here, and the
// one rename in that fold is worth knowing about: this client's -thinking
// becomes Show, which decides what the transcript displays and nothing else:
// the model reasons, is billed, and replays its reasoning either way.
func build(config Config, local []nacelle.Tool, approve nacelle.Approve, hooks map[nacelle.HookPoint][]nacelle.Hook) (*nacelle.Agent, nacelle.Backend, error) {
	backend, err := chosen(config)
	if err != nil {
		return nil, nil, err
	}

	retrying := nacelle.Retry(backend, nacelle.RetryOptions{})
	local, err = withSubagents(config, retrying, local, approve)
	if err != nil {
		return nil, nil, err
	}
	local = withTasks(local)

	agent, err := nacelle.New(nacelle.Config{
		Backend: retrying,
		System:  config.System,
		Thinking: nacelle.Thinking{
			Effort: nacelle.Effort(config.Effort),
			Budget: *config.Budget,
			Show:   *config.Thinking,
		},
		Tools:         local,
		MaxIterations: *config.MaxIterations,
		Approve:       approve,
		Hooks:         hooks,
	})
	if err != nil {
		return nil, nil, err
	}
	return agent, backend, nil
}

// chosen builds the backend the settings ask for.
//
// An unknown name is refused rather than quietly falling back to a model the
// caller did not choose and will be billed for, which is the same reason the
// library itself ships no default backend.
func chosen(config Config) (nacelle.Backend, error) {
	switch config.Backend {
	case "anthropic":
		return anthropic.New(anthropic.Config{Model: config.Model}), nil
	case "openrouter":
		if config.Model == "" {
			return nil, fmt.Errorf("openrouter needs a model: pass -model, or set model in ~/%s", ConfigFile)
		}
		return openrouter.New(openrouter.Config{Model: config.Model})
	default:
		return nil, fmt.Errorf("unknown backend %q, want anthropic or openrouter", config.Backend)
	}
}
