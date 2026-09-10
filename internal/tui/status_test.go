package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

func TestTheSpinnerAndTheWaitingPhraseAreOneColouredStatement(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()

	line, _, _ := strings.Cut(m.status(), "\n")
	if strings.HasPrefix(line, visible(line)) {
		t.Fatalf("status = %q, want the whole line coloured rather than left plain", line)
	}
	if opens := strings.Index(line, "\x1b["); opens != 0 {
		t.Errorf("status = %q, want the colour to open before the spinner, not after it", line)
	}
	if inner := strings.Count(line, "\x1b["); inner != 2 {
		t.Errorf("status = %q, want one colour and one reset, not %d sequences", line, inner)
	}
}

func TestTheStatusColourSaysWhichPhaseTheRunIsIn(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()
	idle := m.status()

	for _, name := range []string{"run_command", "some_mcp_tool"} {
		m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: name, Input: `{}`}, false)
		busy := m.status()

		if colourOf(busy) == "" {
			t.Errorf("status running %q = %q, want a colour rather than a plain line", name, busy)
		}
		if colourOf(busy) == colourOf(idle) {
			t.Errorf("status running %q wears the waiting colour, so the phase change is invisible", name)
		}
	}
}

func colourOf(line string) string {
	if !strings.HasPrefix(line, "\x1b[") {
		return ""
	}
	sequence, _, _ := strings.Cut(line, "m")
	return sequence
}

func TestCostInStatusLine(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100, Cost: 0.0123}

	status := visible(m.status())
	if !strings.Contains(status, "$0.0123") {
		t.Fatalf("status = %q, want the reported cost in it", status)
	}
	if strings.Index(status, "$0.0123") > strings.Index(status, "↑2.6k") {
		t.Errorf("status = %q, want the cost ahead of the counts it summarises", status)
	}

	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100}
	if status := visible(m.status()); strings.Contains(status, "$") {
		t.Errorf("status = %q, want no currency figure when the backend reported none", status)
	}
}

func TestTheElapsedTimerMeasuresTheRunAndDisappearsWithIt(t *testing.T) {
	m := bareBanner()
	m.began = time.Now().Add(-41 * time.Minute)
	m.run.began = time.Now().Add(-12 * time.Second)
	m.run.busy = true

	if got := m.status(); !strings.Contains(got, "12s") || strings.Contains(got, "41m") {
		t.Errorf("running status = %q, want 12s and not 41m", got)
	}

	m.run.busy = false
	if got := m.status(); strings.Contains(got, "12s") {
		t.Errorf("idle status = %q, want no timer left standing", got)
	}

	m.run.busy = true
	m.run.began = time.Time{}
	if got := m.ongoing(); got != "" {
		t.Errorf("ongoing with zero stamp = %q, want empty", got)
	}
}

func TestStatusSeparatesInputFromOutputTokens(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{
		InputTokens:     2600,
		OutputTokens:    1100,
		CacheReadTokens: 9800,
	}

	got := visible(m.View().Content)
	for _, want := range []string{"↑2.6k", "↓1.1k"} {
		if !strings.Contains(got, want) {
			t.Errorf("status %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "3700 tokens") || strings.Contains(got, "13.5k tokens") {
		t.Errorf("status still shows one merged total: %q", got)
	}
	if strings.Contains(got, "cached") {
		t.Errorf("status still shows cached tokens: %q", got)
	}
}

func TestAskingShowsTheSpinnerBeforeAnythingArrives(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.prompt.SetValue("are you there?")
	m.ask()
	defer m.run.cancel()

	if !strings.Contains(visible(m.status()), "waiting ") {
		t.Errorf("status = %q, want it saying so while nothing has arrived yet", visible(m.status()))
	}

	m.consume(result{event: nacelle.Event{Kind: nacelle.KindText, Text: "h"}})
	if !m.run.busy {
		t.Fatal("the run stopped being busy on its first event")
	}
	msg, ok := m.spin.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := m.spun(msg); cmd == nil {
		t.Error("the spinner stopped re-arming after the first event")
	}
}

const long = "this is a deliberately long question, far wider than the window it is " +
	"being typed into, so a prompt that refuses to wrap has nowhere to put it"

func TestPromptWrappingAndGrowth(t *testing.T) {
	m := sized()
	m.prompt.SetValue(long)

	if handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Error("up was claimed by the client, want it left to the prompt")
	}
	if got := m.prompt.Height(); got < 2 {
		t.Fatalf("prompt height = %d, want it grown past one row", got)
	}
	if !strings.Contains(visible(m.prompt.View()), "this is a deliberately long question") {
		t.Errorf("prompt = %q, want the question actually shown", visible(m.prompt.View()))
	}

	before := m.liveRows
	m.layout(m.windowHeight)
	grew := m.prompt.Height() - 1
	if got := m.liveRows; got != before-grew {
		t.Errorf("live rows = %d, want %d", got, before-grew)
	}

	m.prompt.SetValue(strings.Repeat("word ", 400))
	if got := m.prompt.Height(); got > promptRows {
		t.Errorf("prompt height = %d, want no more than cap %d", got, promptRows)
	}
	verifyPromptClearedRestoresLiveRows(t, m)
}

func verifyPromptClearedRestoresLiveRows(t *testing.T, m *Model) {
	t.Helper()
	m.prompt.Reset()
	m.layout(m.windowHeight)
	tall := m.liveRows
	m.agent = answering(t)
	m.prompt.SetValue(long)
	m.layout(m.windowHeight)
	m.ask()
	defer m.run.cancel()
	if got := m.liveRows; got != tall {
		t.Errorf("live rows = %d, want %d back after prompt cleared", got, tall)
	}
}

func TestPromptContinuationsAndBounds(t *testing.T) {
	prompt := continuation("| ")
	if got := visible(prompt(textarea.PromptInfo{LineNumber: 0, Focused: true})); got != "|  " {
		t.Errorf("first row marker = %q, want the prefix '| ' plus a margin space", got)
	}
	if got := prompt(textarea.PromptInfo{LineNumber: 1, Focused: true}); strings.TrimSpace(visible(got)) != "" {
		t.Errorf("wrapped row marker = %q, want only the indent", got)
	}
	if got := continuation("")(textarea.PromptInfo{LineNumber: 1, Focused: true}); got != " " {
		t.Errorf("empty prefix wrapped row marker = %q, want a one-space gutter", got)
	}

	for _, height := range []int{6, 8, 12, 24} {
		m := sized()
		m.resize(tea.WindowSizeMsg{Width: 60, Height: height})
		m.prompt.SetValue(strings.Repeat("word ", 300))
		m.layout(m.windowHeight)

		if got := strings.Count(m.View().Content, "\n") + 1; got > height {
			t.Errorf("window %d: View drew %d rows, want <= %d", height, got, height)
		}
	}
}

func TestPromptHistoryNavigation(t *testing.T) {
	m := sized()
	if handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Error("up was claimed with an empty history")
	}

	m.hist.Remember("first question", m.Items())
	m.hist.Remember("second question", m.Items())
	m.hist.Remember("first question", m.Items())

	if got := strings.Join(m.hist.Past, "|"); got != "second question|first question" {
		t.Fatalf("history = %q, want duplicate moved to end", got)
	}

	for _, want := range []string{"first question", "second question"} {
		m.key(tea.KeyPressMsg{Code: tea.KeyUp})
		if got := m.prompt.Value(); got != want {
			t.Fatalf("prompt = %q, want %q", got, want)
		}
	}

	m2 := sized()
	m2.hist.Remember("an earlier question", m2.Items())
	m2.prompt.SetValue("half a thought")
	m2.key(tea.KeyPressMsg{Code: tea.KeyUp})
	m2.key(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m2.prompt.Value(); got != "half a thought" {
		t.Fatalf("prompt = %q, want draft restored", got)
	}
}
