package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) resumeCmd() tea.Cmd {
	projectRoot := m.run.root
	if projectRoot == "" {
		projectRoot = "."
	}
	sessionFiles := listSessionFiles(projectRoot)
	if len(sessionFiles) == 0 {
		m.say(fromClient, "no previous sessions found for this project")
		return nil
	}
	mostRecent := sessionFiles[0]
	conversation := loadSession(mostRecent)
	if conversation == nil {
		m.say(fromClient, "failed to load session: "+mostRecent)
		return nil
	}
	m.conversation = conversation
	m.say(fromClient, fmt.Sprintf("resumed session from %s (%d messages)",
		filepath.Base(mostRecent), len(conversation)))
	return nil
}

func (m *Model) sessionsCmd() tea.Cmd {
	projectRoot := m.run.root
	if projectRoot == "" {
		projectRoot = "."
	}
	sessionFiles := listSessionFiles(projectRoot)
	if len(sessionFiles) == 0 {
		m.say(fromClient, "no previous sessions found for this project")
		return nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("sessions for project %s:", projectRoot))
	for _, filePath := range sessionFiles {
		lines = append(lines, formatSessionEntry(filePath))
	}
	m.say(fromClient, strings.Join(lines, "\n"))
	return nil
}
