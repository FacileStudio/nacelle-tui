package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/tui/approval"
)

func (m *Model) decide(press tea.KeyPressMsg) tea.Cmd {
	var decision approval.Decision
	switch press.String() {
	case "y":
		decision = approval.AllowedOnce
	case "a":
		decision = approval.AllowedForSession
	case "n":
		decision = approval.Denied
	default:
		return nil
	}

	pending := m.run.pending
	m.run.pending = nil
	pending.Decision <- decision
	return nil
}

// BuildApprovals constructs the approval gate and returns the approval function.
func BuildApprovals(config Config) (*Approvals, nacelle.Approve) {
	return approval.Build(*config.ApproveTools)
}

// WireApprovals connects a non-nil gate to the running program's Send.
func WireApprovals(gate *Approvals, program *tea.Program) {
	if gate != nil {
		gate.Wire(program.Send)
	}
}
