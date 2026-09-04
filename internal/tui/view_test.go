package tui

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

func called(id, name, input string) nacelle.Event {
	return nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: id, Name: name, Input: input}}
}

func printsDirectly(file *ast.File) bool {
	var found bool
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Println" {
			return true
		}
		pkg, isIdent := selector.X.(*ast.Ident)
		found = found || (isIdent && pkg.Name == "tea")
		return true
	})
	return found
}

func TestACallSaysNothingUntilItsResultArrives(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(called("1", "read_file", `{"path":"view.go"}`))

	if lines := spoken(m); len(lines) != 0 {
		t.Errorf("transcript = %v, want empty", lines)
	}
	if status := visible(m.status()); !strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want running read_file", status)
	}
}

func TestAFailedToolKeepsItsCallLineAndItsError(t *testing.T) {
	m := sized()
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID:       "1",
		Name:     "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 1 {
		t.Fatalf("transcript = %v, want 1 entry", lines)
	}
	if !strings.Contains(lines[0], "run_command(go build ./...)") {
		t.Errorf("call line = %q", lines[0])
	}
	if !strings.Contains(lines[0], "exit status 2") || !strings.Contains(lines[0], "12ms") {
		t.Errorf("failure line = %q", lines[0])
	}
}

func TestIdenticalFailuresCollapse(t *testing.T) {
	m := sized()
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.absorb(called("2", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "2", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 15 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 1 {
		t.Fatalf("transcript = %v, want 1 entry", lines)
	}
	if !strings.Contains(lines[0], "run_command(go build ./...)") {
		t.Errorf("call line = %q", lines[0])
	}
	if !strings.Contains(lines[0], "2 times") {
		t.Errorf("failure = %q, want 2 times count", lines[0])
	}
}

func TestDifferentFailuresAreNotCollapsed(t *testing.T) {
	m := sized()
	m.absorb(called("1", "run_command", `{"command":"go build ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "run_command",
		Err:      errors.New("exit status 2"),
		Duration: 12 * time.Millisecond,
	}})
	m.absorb(called("2", "run_command", `{"command":"go test ./..."}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "2", Name: "run_command",
		Err:      errors.New("test failure"),
		Duration: 20 * time.Millisecond,
	}})
	m.stranded()

	lines := spoken(m)
	if len(lines) != 2 {
		t.Fatalf("transcript = %v, want 2 entries", lines)
	}
}

func TestADiscardedCallDropsItsHeldLine(t *testing.T) {
	m := sized()
	m.absorb(called("1", "read_file", `{"path":"view.go"}`))
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "read_file", Discarded: true,
	}})

	if lines := spoken(m); len(lines) != 0 {
		t.Errorf("transcript = %v, want 0 lines", lines)
	}
	if n := m.running(); n != 0 {
		t.Errorf("running = %v, want 0", n)
	}
}

func TestARunThatEndsMidToolStillSaysWhatItAskedFor(t *testing.T) {
	m := sized()
	m.run.cancel = func() {}
	m.run.busy = true
	m.absorb(called("a", "read_file", `{"path":"view.go"}`))
	m.absorb(called("b", "run_command", `{"command":"go test ./..."}`))

	m.settle()

	lines := spoken(m)
	if len(lines) != 2 {
		t.Fatalf("transcript = %v, want 2 calls", lines)
	}
	if !strings.Contains(lines[0], "read_file(view.go)") || !strings.Contains(lines[1], "run_command(go test ./...)") {
		t.Errorf("transcript = %v", lines)
	}
}

func TestABankRowSeparatesTheAnswerFromTheStatusLine(t *testing.T) {
	m := sized()

	lines := strings.Split(visible(m.View().Content), "\n")
	status := -1
	for i, line := range lines {
		if strings.Contains(line, "ready") {
			status = i
			break
		}
	}
	if status < 1 {
		t.Fatalf("no status line in\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(lines[status-1]) != "" {
		t.Errorf("row above status = %q, want blank", lines[status-1])
	}
}

func TestAnEndedRunSaysTheAnswerBeforeTheToolItWasStillHolding(t *testing.T) {
	m := bareBanner()
	m.width = 80
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "reading the file now"})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{
		ID: "1", Name: "read_file", Input: `{"path":"view.go"}`,
	}})
	m.unprinted = nil

	m.settle()

	said := strings.Join(m.unprinted, "\n")
	answer := strings.Index(said, "reading the file")
	call := strings.Index(said, "read_file(")
	if answer < 0 || call < 0 {
		t.Fatalf("missing answer or call: %q", said)
	}
	if answer > call {
		t.Errorf("tool line above sentence")
	}
}

func TestAHeightOnlyResizeDoesNotRebuildTheMarkdownRenderer(t *testing.T) {
	m := sized()
	before := m.pretty
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 30})
	if m.pretty != before {
		t.Error("markdown renderer rebuilt for height-only resize")
	}
}

func TestAWidthChangeRebuildsTheMarkdownRenderer(t *testing.T) {
	m := sized()
	before := m.pretty
	m.resize(tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.pretty == before {
		t.Error("markdown renderer not rebuilt for width resize")
	}
}

func TestTheFrameNeverFillsTheWindow(t *testing.T) {
	m := sized()
	frame := m.liveRows + 2 + m.prompt.Height() + m.menu.Height() + m.Height(m.editing())
	if frame >= m.windowHeight {
		t.Errorf("frame is %d rows, want room left", frame)
	}
}

func TestOnlyPrintedHandsABatchToTheTerminal(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}
	set := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || name == "view.go" {
			continue
		}
		parsed, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		if printsDirectly(parsed) {
			t.Errorf("%s calls tea.Println directly", name)
		}
	}
}
