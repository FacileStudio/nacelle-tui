package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
)

// tuiModel is a model switched into alternate-screen mode with a window, the
// arrangement the mode setting produces in Launch.
func tuiModel() *Model {
	m := NewModel(nil, "banner", nil, SessionConfig{CompactAt: 100_000, PromptPrefix: "| ", PromptPlaceholder: "placeholder"})
	m.mode = modeTUI
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

// renderMode is the one seam the mode string crosses: the config names it,
// this names the renderer. Anything that is not "tui" is inline by design.
func TestRenderModeParsesTheString(t *testing.T) {
	if renderMode("tui") != modeTUI {
		t.Error("renderMode(\"tui\") != modeTUI")
	}
	if renderMode("inline") != modeInline {
		t.Error("renderMode(\"inline\") != modeInline")
	}
	if renderMode("bogus") != modeInline {
		t.Error("an unknown mode should fall back to inline")
	}
}

// Inside the alternate screen there is no terminal scrollback for Println to
// reach, so a finished line is held in memory and drawn back into the view, and
// nothing is handed to tea as a Cmd. The same say-path must produce a printed
// command in the inline mode it shares the queue with.
func TestTUIModeHoldsPrintedLinesInsteadOfHandingThemToTheTerminal(t *testing.T) {
	m := tuiModel()
	m.say(fromReader, "a question")

	if cmd := m.prints(); cmd != nil {
		t.Error("tui mode handed a Cmd to the terminal, want it held in memory")
	}
	held := visible(strings.Join(m.hold, "\n"))
	if !strings.Contains(held, "a question") {
		t.Errorf("held = %q, want the said line", held)
	}
	if len(m.unprinted) != 0 {
		t.Errorf("unprinted = %v, want it drained into hold", m.unprinted)
	}
}

// The hold is a bounded ring of painted rows: past holdRowsCap the oldest rows
// drop, so a long session cannot charge every frame for rows no frame can
// show, and the newest rows — the ones a frame can show — always survive.
func TestTheHoldDropsItsOldestRowsOnceCapped(t *testing.T) {
	m := tuiModel()
	for i := range 2 * holdRowsCap {
		m.printed(fmt.Sprintf("row %d", i))
	}
	if len(m.hold) != holdRowsCap {
		t.Fatalf("hold = %d rows, want it capped at %d", len(m.hold), holdRowsCap)
	}
	if want := fmt.Sprintf("row %d", holdRowsCap); m.hold[0] != want {
		t.Errorf("first held row = %q, want the first row past the cap %q", m.hold[0], want)
	}
	if want := fmt.Sprintf("row %d", 2*holdRowsCap-1); m.hold[len(m.hold)-1] != want {
		t.Errorf("last held row = %q, want the newest row %q", m.hold[len(m.hold)-1], want)
	}
}

// The whole point of the mode: the prompt is pinned to the bottom of an
// alternate screen with exactly one blank row above and one below it, like a
// vim status bar, instead of riding the live region up and down.
// The prompt is pinned: a single blank row sits at the very bottom, the rows
// between the prompt's text and it are the prompt's own y-padding, and one blank
// row separates the transcript from the prompt above it.
func TestTUIModePinsThePromptToTheBottom(t *testing.T) {
	m := tuiModel()
	m.say(fromReader, "a line of transcript")
	m.say(fromModel, "and another line")

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	first, last := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "placeholder") {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		t.Fatalf("no prompt line in\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Errorf("row below prompt = %q, want the single bottom blank", lines[len(lines)-1])
	}
	for i := last + 1; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			t.Errorf("prompt padding row %d = %q, want blank", i, lines[i])
		}
	}
	if strings.TrimSpace(lines[first-1]) != "" {
		t.Errorf("row above prompt = %q, want blank", lines[first-1])
	}
}

// In the alternate screen the whole frame is ours; the AltScreen flag is what
// tells Bubble Tea to draw it there rather than over scrollback.
func TestTUIModeRendersOnTheAlternateScreen(t *testing.T) {
	m := tuiModel()
	if !m.View().AltScreen {
		t.Error("tui mode view is not marked for the alternate screen")
	}
}

// Inline mode must keep living in the terminal's own scrollback: the same
// say-then-print path yields a Cmd here, never a held buffer.
func TestInlineModeHandsTheTranscriptToTheTerminal(t *testing.T) {
	m := sized()
	m.say(fromReader, "a question")

	if cmd := m.prints(); cmd == nil {
		t.Error("inline mode produced no Cmd, want the line handed to the terminal")
	}
	if len(m.hold) != 0 {
		t.Errorf("hold = %v, want it empty in inline mode", m.hold)
	}
}

// The prompt reads as a roomier bar rather than a cramped line: even with a
// single line of input it keeps its MinHeight floor, which is the y-padding
// that makes the field taller than one row.
func TestThePromptHasVerticalPadding(t *testing.T) {
	m := sized()
	lines := strings.Split(ansi.Strip(m.prompt.View()), "\n")
	if len(lines) < minHeightRows {
		t.Errorf("prompt rendered %d rows for one line of input, want at least %d", len(lines), minHeightRows)
	}
	if aNonBlankRow(lines) < 0 {
		t.Errorf("no text row found in\n%s", strings.Join(lines, "\n"))
	}
}

// aNonBlankRow is the index of the first row with visible text, or -1 when the
// screen is all blank.
func aNonBlankRow(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			return i
		}
	}
	return -1
}
