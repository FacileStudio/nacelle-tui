package main

import (
	"testing"
)

// Off until it proves itself: delegation spends a nested run's tokens on a
// bet the person running this has not placed yet.
func TestSubagentsDefaultOff(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Subagents {
		t.Error("subagents default on")
	}
}

// The whole precedence chain has to carry the toggle, or the layer that
// turns it on is not the layer that decides.
func TestSubagentsFollowThePrecedenceChain(t *testing.T) {
	written(t, "subagents: false")
	t.Setenv("NACELLE_SUBAGENTS", "true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Subagents {
		t.Error("the environment did not beat the file")
	}
}
