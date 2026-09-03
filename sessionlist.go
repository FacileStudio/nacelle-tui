package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FacileStudio/nacelle"
)

// listSessionFiles returns a slice of session file paths sorted by modification time (newest first)
// for the given project root. If projectRoot is empty, it lists all sessions.
// If the sessions directory doesn't exist, it returns an empty slice.
func listSessionFiles(projectRoot string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	sessionsDir := filepath.Join(home, ".nacelle", "sessions")

	if projectRoot != "" {
		cleanRoot := filepath.Clean(projectRoot)
		switch cleanRoot {
		case ".":
			cleanRoot = ""
		case "..":
			cleanRoot = filepath.Base(cleanRoot)
		}
		if cleanRoot != "" {
			sessionsDir = filepath.Join(sessionsDir, cleanRoot)
		}
	}

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	var sessionFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".jsonl") {
			sessionFiles = append(sessionFiles, filepath.Join(sessionsDir, f.Name()))
		}
	}

	sort.Slice(sessionFiles, func(i, j int) bool {
		infoI, errI := os.Stat(sessionFiles[i])
		infoJ, errJ := os.Stat(sessionFiles[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return sessionFiles
}

// loadSession loads and parses a session file, returning the conversation.
// It returns nil if the file cannot be read or parsed.
func loadSession(path string) []nacelle.Message {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var conversation []nacelle.Message
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")

	startIndex := 0
	if len(lines) > 0 {
		var header sessionHeader
		if err := json.Unmarshal([]byte(lines[0]), &header); err == nil && header.Version == 1 {
			startIndex = 1
		}
	}

	for _, line := range lines[startIndex:] {
		if line == "" {
			continue
		}
		var entry sessionEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		switch entry.Who {
		case "question":
			conversation = append(conversation, nacelle.UserText(entry.Text))
		case "answer":
			conversation = append(conversation, nacelle.AssistantText(entry.Text))
		}
	}

	return conversation
}

func formatSessionEntry(filePath string) string {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Sprintf("  %s (error reading file)", filepath.Base(filePath))
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("  %s (unreadable)", filepath.Base(filePath))
	}

	linesData := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(linesData) == 0 {
		return fmt.Sprintf("  %s · %s",
			filepath.Base(filePath),
			info.ModTime().Format("2006-01-02 15:04:05"))
	}

	var header sessionHeader
	if err := json.Unmarshal([]byte(linesData[0]), &header); err == nil && header.Version == 1 {
		return fmt.Sprintf("  %s · %s · %s · %s",
			filepath.Base(filePath),
			header.Backend,
			header.Model,
			header.Started[:19])
	}

	return fmt.Sprintf("  %s · %s",
		filepath.Base(filePath),
		info.ModTime().Format("2006-01-02 15:04:05"))
}
