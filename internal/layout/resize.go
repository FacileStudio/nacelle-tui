package layout

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Budget calculates how many rows a single print operation may scroll.
func Budget(windowHeight, frameRows int) int {
	return max(windowHeight-frameRows, 1)
}

// Scrolls calculates the number of terminal rows occupied by line at width.
func Scrolls(line string, width int) int {
	w := max(width, 1)
	if cells := ansi.StringWidth(line); cells > w {
		return 1 + cells/w
	}
	return 1
}

// Fits calculates how many lines fit within the given budget and width.
func Fits(lines []string, budget, width int) int {
	rows, taken := 0, 0
	for _, line := range lines {
		rows += Scrolls(line, width)
		if rows > budget && taken > 0 {
			break
		}
		taken++
	}
	return taken
}

// Batches splits text into batches that fit within budget rows.
func Batches(text string, budget, width int) []string {
	lines := strings.Split(text, "\n")
	var batches []string
	for len(lines) > 0 {
		take := Fits(lines, budget, width)
		batches = append(batches, strings.Join(lines[:take], "\n"))
		lines = lines[take:]
	}
	return batches
}

// LiveRows computes the available rows for live streaming output.
func LiveRows(height, taken int) int {
	return max(height-taken-height/2, 1)
}

// PromptRows is the ceiling on how tall the prompt textarea may grow.
const PromptRows = 10

// PromptCap calculates the maximum height allowed for prompt input in a window.
func PromptCap(height int) int {
	return max(1, min(PromptRows, (height-3)/2))
}
