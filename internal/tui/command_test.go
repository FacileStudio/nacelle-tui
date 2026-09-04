package tui

import (
	"context"
	"iter"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle-tui/internal/tui/menu"
)

// insertPick replaces the whole value — it's for start-of-line only.
func TestInsertPickReplacesWholeValue(t *testing.T) {
	want := "/clear "
	if got := menu.InsertPick("anything", "/clear"); got != want {
		t.Errorf("insertPick = %q, want %q", got, want)
	}
}

// The selected row highlights via theme.question, which carries Padding(0, 1)
// for its other job — the transcript's quoted-question pill. Reusing it
// unstripped here shifted only the selected row's text one column right, so
// whichever row happened to be selected read as " /clear" instead of
// "/clear" while every unselected row below it did not.
func TestViewMenuSelectedRowAlignsWithUnselectedRows(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	first := strings.Split(visible(m.viewMenu()), "\n")[0]
	if !strings.HasPrefix(first, "→ /clear") {
		t.Errorf("first row = %q, want the selected row marked with an arrow", first)
	}

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	lines := strings.Split(visible(m.viewMenu()), "\n")
	if !strings.HasPrefix(lines[0], "  /clear") {
		t.Errorf("first row = %q after moving selection away, want it indented to the arrow", lines[0])
	}
	if !strings.HasPrefix(lines[1], "→ /cost") {
		t.Errorf("second row = %q once selected, want the arrow on it", lines[1])
	}
}

// TestViewMenuListsEveryVisibleMatch verifies that all built-in commands
// appear in the dropdown menu including /status.
func TestViewMenuListsEveryVisibleMatch(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	got := m.viewMenu()

	for _, want := range []string{"/clear", "/help", "/quit", "/status"} {
		if !strings.Contains(got, want) {
			t.Errorf("viewMenu() = %q, want it to mention %q", got, want)
		}
	}
}

// key() is where up/down actually get routed — this is the one place that
// proves the menu wins over the scroller while it's open, not only that
// navigateMenu itself works in isolation.
func TestKeyRoutesUpDownToTheMenuInsteadOfScrollingWhileItIsOpen(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyDown})

	if m.menu.Selected == 0 {
		t.Error("selected did not move, want key() to have routed down to the menu")
	}
	if !handled {
		t.Error("down fell through to the prompt, want it claimed by the open menu instead")
	}
}

func TestSlashCostReportsTheSessionTotal(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 400, OutputTokens: 100, Cost: 0.01}
	m.run.usage = nacelle.Usage{InputTokens: 600, OutputTokens: 900,
		CacheReadTokens: 2000, Cost: 0.02}
	m.tools, m.failed = 3, 1

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if !strings.Contains(said, "in 1.0k") || !strings.Contains(said, "out 1.0k") {
		t.Errorf("said = %q, want the merged token totals", said)
	}
	if !strings.Contains(said, "2.0k cached") {
		t.Errorf("said = %q, want the cache reads", said)
	}
	if !strings.Contains(said, "$0.0300") {
		t.Errorf("said = %q, want spent plus the run in flight", said)
	}
	if !strings.Contains(said, "3 tools · 1 failed") {
		t.Errorf("said = %q, want the tool counts", said)
	}
}

func TestSlashCostOnAnEmptySession(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if strings.Contains(said, "$") || strings.Contains(said, "tool") {
		t.Errorf("said = %q, want no cost and no tools for an unused session", said)
	}
	if !strings.Contains(said, "session · ") {
		t.Errorf("said = %q, want the session line regardless", said)
	}
}

func TestSlashCostDurationsFromSessionStart(t *testing.T) {
	m := sized()
	m.began = time.Now().Add(-90 * time.Second)

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if !strings.Contains(said, "session · 1m30s") {
		t.Errorf("said = %q, want the session's own span", said)
	}
}

type answeringStub struct{ received nacelle.Request }

func (s *answeringStub) Name() string                       { return "stub" }
func (s *answeringStub) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }
func (s *answeringStub) Stream(_ context.Context, request nacelle.Request) iter.Seq2[nacelle.Event, error] {
	s.received = request
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindDone}, nil)
	}
}
func (s *answeringStub) CountTokens(_ context.Context, request nacelle.Request) (int64, error) {
	s.received = request
	return 0, nil
}

func TestSlashSkillStartsARunWithTheSkillsBodyAsTheQuestion(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "Name: deploy\ndescription: ships the app")
	s := skill{Name: "deploy", Path: filepath.Join(dir, "SKILL.md")}

	agent, err := nacelle.New(nacelle.Config{Backend: &answeringStub{}, System: "test"})
	if err != nil {
		t.Fatalf("nacelle.New: %v", err)
	}
	m := newModel(agent, "test · model", []skill{s}, int64(100_000), false)
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 24})
	t.Cleanup(m.run.cancel)

	m.prompt.SetValue("/skill:deploy to staging")
	m.ask()

	if !m.run.busy {
		t.Fatal("/skill:deploy did not start a run")
	}
	if len(m.conversation) != 1 {
		t.Fatalf("conversation = %v, want exactly the expanded skill sent", m.conversation)
	}
	sent := m.conversation[0].Parts[0].(nacelle.Text).Text
	if !strings.Contains(sent, "Do the thing.") || !strings.HasSuffix(sent, "User: to staging") {
		t.Errorf("sent = %q, want the skill body plus the args as a User: line", sent)
	}
}

func TestSlashSkillReportsAnUnknownSkillWithoutStartingARun(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/skill:nope")

	printed := printedBy(m.ask())

	if m.run.busy {
		t.Error("an unknown skill started a run")
	}
	echo, reply := strings.Index(printed, "/skill:nope"), strings.Index(printed, "unknown skill")
	if reply < 0 || !strings.Contains(printed, "nope") {
		t.Fatalf("printed = %q, want a line naming the unknown skill", printed)
	}
	if echo < 0 || echo > reply {
		t.Errorf("printed = %q, want the echoed input above the reply to it", printed)
	}
}
