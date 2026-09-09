package settings

import (
	"testing"
)

// On by default: delegation is enabled out of the box.
func TestSubagentsDefaultOn(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Subagents {
		t.Error("subagents default off")
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

// The settings layer can only resolve the yaml into the config; mounting the
// tool is agent wiring, asserted in the agent package's delegate_test.go. What
// the layer owes that mount is that subagents: true reaches the resolved
// config.
func TestSubagentsTrueInTheFileTurnsTheMountOn(t *testing.T) {
	written(t, "subagents: true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Subagents {
		t.Error("subagents: true in the file left the mount off")
	}
}
