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
	"github.com/FacileStudio/nacelle-tui/internal/menu"
)

func TestMenuItemsListsCommandsBeforeSkillsWithDescriptions(t *testing.T) {
	items := menuItems(map[string]skill{"deploy": {Name: "deploy", Description: "ships the app"}})

	if len(items) != len(commands)+1 {
		t.Fatalf("menuItems = %+v, want every command plus the one skill", items)
	}
	last := items[len(items)-1]
	if last.Value != "/skill:deploy" || last.Description != "ships the app" {
		t.Errorf("last item = %+v, want the skill, with its own description", last)
	}
	for _, it := range items[:len(items)-1] {
		if it.Description != "" {
			t.Errorf("command %q carried a description %q, want none", it.Value, it.Description)
		}
	}
}

func TestRefreshMenuFiltering(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	if !m.menu.Open() || len(m.menu.Filtered) != len(commands) {
		t.Errorf("expected open with all commands on '/'")
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	if len(m.menu.Filtered) != 1 || m.menu.Filtered[0].Value != "/clear" {
		t.Errorf("filtered = %+v, want /clear", m.menu.Filtered)
	}

	m.prompt.SetValue("/clear")
	m.refreshMenu()
	if m.menu.Open() {
		t.Error("menu stayed open on fully typed command")
	}

	m.menu.Items = []menu.Item{{Value: "/skill:review"}, {Value: "/skill:review-pr"}}
	m.prompt.SetValue("/skill:review-")
	m.refreshMenu()
	if !m.menu.Open() {
		t.Error("menu did not reopen when typing past exact match")
	}

	m.prompt.SetValue("hello")
	m.refreshMenu()
	if m.menu.Open() {
		t.Error("menu stayed open when line no longer started with slash")
	}
}

func TestKeyEnterSendsAFullyTypedCommandInsteadOfRepickingIt(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/clear")
	m.refreshMenu()

	m.key(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := m.prompt.Value(); got != "" {
		t.Errorf("prompt = %q after enter, want empty", got)
	}
}

func TestNavigateMenuSelectionTabAndEsc(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.menu.Selected != 0 {
		t.Errorf("selected = %d after up at top", m.menu.Selected)
	}

	for range m.menu.Filtered {
		m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.menu.Selected != len(m.menu.Filtered)-1 {
		t.Errorf("selected = %d after down to end", m.menu.Selected)
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	handled, _ := m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyTab})
	if !handled || m.prompt.Value() != "/clear " || m.run.busy || m.menu.Open() {
		t.Errorf("tab failed: handled=%v prompt=%q busy=%v open=%v", handled, m.prompt.Value(), m.run.busy, m.menu.Open())
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	handled, _ = m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || m.prompt.Value() != "/cl" || m.menu.Open() {
		t.Errorf("esc failed: handled=%v prompt=%q open=%v", handled, m.prompt.Value(), m.menu.Open())
	}
}

func TestViewMenuRendering(t *testing.T) {
	m := sized()
	if got := m.viewMenu(); got != "" {
		t.Errorf("viewMenu() = %q, want empty when closed", got)
	}

	m.prompt.SetValue("/")
	m.refreshMenu()

	first := strings.Split(visible(m.viewMenu()), "\n")[0]
	if !strings.HasPrefix(first, "→ /clear") {
		t.Errorf("first row = %q, want selected marker", first)
	}

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	lines := strings.Split(visible(m.viewMenu()), "\n")
	if !strings.HasPrefix(lines[0], "  /clear") || !strings.HasPrefix(lines[1], "→ /cost") {
		t.Errorf("selection move failed: lines[0]=%q lines[1]=%q", lines[0], lines[1])
	}

	got := m.viewMenu()
	for _, want := range []string{"/clear", "/help", "/quit", "/status"} {
		if !strings.Contains(got, want) {
			t.Errorf("viewMenu() missing %q", want)
		}
	}
}

func TestKeyRoutesUpDownToTheMenuInsteadOfScrollingWhileItIsOpen(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyDown})

	if m.menu.Selected == 0 || !handled {
		t.Error("down was not handled by open menu")
	}
}

func TestSlashCostReportsTheSessionTotal(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 400, OutputTokens: 100, Cost: 0.01}
	m.run.usage = nacelle.Usage{InputTokens: 600, OutputTokens: 900, CacheReadTokens: 2000, Cost: 0.02}
	m.tools, m.failed = 3, 1
	m.began = time.Now().Add(-90 * time.Second)

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	for _, want := range []string{"in 1.0k", "out 1.0k", "2.0k cached", "$0.0300", "3 tools · 1 failed", "session · 1m30s"} {
		if !strings.Contains(said, want) {
			t.Errorf("said = %q, want %q", said, want)
		}
	}
}

func TestSlashCostOnAnEmptySession(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if strings.Contains(said, "$") || strings.Contains(said, "tool") {
		t.Errorf("said = %q, want no cost and no tools for empty session", said)
	}
	if !strings.Contains(said, "session · ") {
		t.Errorf("said = %q, want session line", said)
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
		t.Fatalf("conversation = %v, want expanded skill", m.conversation)
	}
	sent := m.conversation[0].Parts[0].(nacelle.Text).Text
	if !strings.Contains(sent, "Do the thing.") || !strings.HasSuffix(sent, "User: to staging") {
		t.Errorf("sent = %q, want skill body + args", sent)
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
		t.Fatalf("printed = %q, want unknown skill error", printed)
	}
	if echo < 0 || echo > reply {
		t.Errorf("printed = %q, want echoed input before reply", printed)
	}
}
