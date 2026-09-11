package tui

import (
	"fmt"

	"github.com/FacileStudio/nacelle-tui/internal/herdr"
	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/usage"
)

// boot wires the session config the model was built from into its live fields —
// the ones that arrive as pointers or are only meaningful on the running model
// rather than at construction.
func boot(m *Model, c UISession) {
	m.groupTools = c.GroupTools != nil && *c.GroupTools
	m.Expanded = c.ShowThinking
	m.run.root = c.Root
	m.run.diffs = c.Diffs
	m.delegate = c.DelegateConfig
	m.mode = renderMode(c.Mode)
	m.sink = usage.NewSink(c.Root, c.Model)
	m.session = sessions.OpenSession(c.Backend, c.Model, c.Root)
	herdr.SetSession(m.herdrClient, m.session.Path())
}

// startupPrint hands the pre-queued banner lines to the terminal — unless it is
// running on the alternate screen, where Println would be a no-op and the lines
// are held to be drawn back into the view instead.
func startupPrint(m *Model) {
	if m.mode == modeTUI {
		return
	}
	for _, line := range m.unprinted {
		fmt.Println(line)
	}
	m.unprinted = nil
}
