package diagnostics

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func render(fs []finding) string {
	if len(fs) == 0 {
		return ""
	}
	if len(fs) > maxFindings {
		return spilled(fs)
	}
	lines := linesOf(fs)
	if joined(lines) <= maxTextBytes {
		return strings.Join(lines, "\n")
	}
	return spilled(fs)
}

func spilled(fs []finding) string {
	name, err := spillFile(strings.Join(linesOf(fs), "\n"))
	pointer := pointerLine(fs, name, err)
	head := fit(linesOf(fs[:min(len(fs), maxFindings-1)]), len(pointer)+1)
	return strings.Join(append(head, pointer), "\n")
}

func pointerLine(fs []finding, name string, err error) string {
	if err != nil {
		return fmt.Sprintf("filet: %d diagnostics; writing the full list failed: %v", len(fs), err)
	}
	return fmt.Sprintf("filet: %d diagnostics; full list: %s", len(fs), name)
}

func fit(lines []string, reserve int) []string {
	for len(lines) > 0 && joined(lines)+reserve > maxTextBytes {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joined(lines []string) int {
	total := 0
	for _, line := range lines {
		total += len(line) + 1
	}
	return max(total-1, 0)
}

func linesOf(fs []finding) []string {
	lines := make([]string, len(fs))
	for i, f := range fs {
		lines[i] = f.line()
	}
	return lines
}

func spillFile(text string) (string, error) {
	sum := sha256.Sum256([]byte(text))
	name := filepath.Join(os.TempDir(), fmt.Sprintf("diagnostics-%x.txt", sum[:6]))
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		return "", err
	}
	return name, nil
}
