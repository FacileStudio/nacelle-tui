package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestGroupOnlyPrintsOnce(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	if m.run.groups[0].count != 3 {
		t.Fatalf("group count = %d, want 3", m.run.groups[0].count)
	}

	said := func() []string { return spoken(m) }

	for _, id := range []string{"a", "b"} {
		m.run.finishTool(nacelle.ToolEvent{ID: id, Name: "read_file", Input: `{"path":"view.go"}`})
		m.finished(&nacelle.ToolEvent{ID: id, Name: "read_file", Input: `{"path":"view.go"}`})
	}
	if lines := said(); len(lines) != 0 {
		t.Errorf("after 2nd finish: %v, want empty transcript", lines)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	lines := said()
	if len(lines) != 1 {
		t.Fatalf("after 3rd finish: %d lines %v, want exactly 1", len(lines), lines)
	}
	if !strings.Contains(lines[0], "☰ 3 reads") {
		t.Errorf("group line = %q, want '☰ 3 reads'", lines[0])
	}
}

func TestGroupIntermediateFailureTrackedAndRendered(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})

	if m.failed != 1 {
		t.Errorf("failed = %d, want 1 after intermediate failure", m.failed)
	}
	if !m.run.groups[0].end.IsZero() {
		t.Errorf("group end should be zero on intermediate finish")
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})

	if m.run.groups[0].end.IsZero() {
		t.Errorf("group end should be set after last finish")
	}
	if !m.run.groups[0].failed {
		t.Errorf("group failed should remain true")
	}
	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "✗ 2 reads") {
		t.Errorf("expected failed group line with ✗, got: %q", joined)
	}
	if !strings.Contains(joined, "file not found") {
		t.Errorf("expected error message in output, got: %q", joined)
	}
}

func TestGroupLastCallFailureRendered(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})

	if m.failed != 0 {
		t.Errorf("failed = %d, want 0 after first successful call", m.failed)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})

	if m.failed != 1 {
		t.Errorf("failed = %d, want 1 after last call failed", m.failed)
	}
	if !m.run.groups[0].failed {
		t.Errorf("group failed should be true")
	}
	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "✗ 2 reads") {
		t.Errorf("expected failed group line with ✗, got: %q", joined)
	}
	if !strings.Contains(joined, "permission denied") {
		t.Errorf("expected error message in output, got: %q", joined)
	}
}

func TestAGroupCallsTrackArgumentsNotNames(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "3", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	g := m.run.groups[0]
	if g.count != 3 {
		t.Errorf("group count = %d, want 3", g.count)
	}
	if len(g.callNames) != 3 {
		t.Fatalf("callNames = %v, want 3 entries", g.callNames)
	}
	want := []string{"view.go", "run.go", "inflight.go"}
	for i, w := range want {
		if g.callNames[i] != w {
			t.Errorf("callNames[%d] = %q, want %q", i, g.callNames[i], w)
		}
	}
}

func TestGroupLineShowsArguments(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{"command":"ls docs"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "run_command", Input: `{"command":"git status"}`}, true)

	line := m.run.groups[0].groupLine(m.width)
	if !strings.Contains(line, "ls docs") {
		t.Errorf("group line = %q, want it to contain 'ls docs'", line)
	}
	if !strings.Contains(line, "git status") {
		t.Errorf("group line = %q, want it to contain 'git status'", line)
	}
	if strings.Count(line, "run_command") > 1 {
		t.Errorf("group line = %q, want the tool name mentioned at most once", line)
	}
}

func TestGroupMultipleDistinctFailures(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})

	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if count := strings.Count(joined, "✗ 2 reads"); count != 1 {
		t.Errorf("expected group line '✗ 2 reads' exactly once, got count %d in %q", count, joined)
	}
	if !strings.Contains(joined, "file not found") {
		t.Errorf("expected 'file not found' in output, got: %q", joined)
	}
	if !strings.Contains(joined, "permission denied") {
		t.Errorf("expected 'permission denied' in output, got: %q", joined)
	}
}

func TestGroupPreservesDiffsOnPartialFailure(t *testing.T) {
	m := sized()
	m.groupTools = true
	if m.run.edits == nil {
		m.run.edits = map[string]editChange{}
	}
	m.run.edits["a"] = editChange{path: "view.go", before: "old", after: "new"}

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`})

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`, Err: errors.New("write error")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`, Err: errors.New("write error")})

	if _, ok := m.run.edits["a"]; ok {
		t.Errorf("edit for call 'a' should be completed and deleted from edits map")
	}
	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "write error") {
		t.Errorf("expected write error in output, got: %q", joined)
	}
}
