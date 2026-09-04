package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func pieces(t *testing.T, cmd tea.Cmd) []string {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice {
		return []string{visible(fmt.Sprint(msg))}
	}
	out := make([]string, 0, value.Len())
	for i := range value.Len() {
		out = append(out, pieces(t, value.Index(i).Interface().(tea.Cmd))...)
	}
	return out
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

func TestAHeightOnlyResizeDoesNotRebuildTheMarkdownRenderer(t *testing.T) {
	m := sized()
	before := m.pretty
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 30})
	if m.pretty != before {
		t.Error("the markdown renderer was rebuilt for a resize that did not change the width")
	}
}

func TestAWidthChangeRebuildsTheMarkdownRenderer(t *testing.T) {
	m := sized()
	before := m.pretty
	m.resize(tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.pretty == before {
		t.Error("the markdown renderer was not rebuilt after the width changed")
	}
}

func TestATallBatchIsCutIntoPiecesTheFrameSurvives(t *testing.T) {
	m := sized()
	m.frameRows = m.windowHeight - 4
	lines := make([]string, 14)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	batches := pieces(t, m.printed(strings.Join(lines, "\n")))
	if len(batches) < 2 {
		t.Fatalf("printed %d batches, want at least 2", len(batches))
	}
	for i, batch := range batches {
		if rows := len(strings.Split(batch, "\n")); rows > m.budget() {
			t.Errorf("batch %d is %d rows, want no more than %d", i, rows, m.budget())
		}
	}
}

func TestABatchThatFitsStaysOneMessage(t *testing.T) {
	m := sized()
	if got := len(pieces(t, m.printed("one\ntwo\nthree"))); got != 1 {
		t.Errorf("printed %d batches, want 1", got)
	}
}

func TestAWrappedLineCostsTheRowsItWrapsOnto(t *testing.T) {
	m := sized()
	m.frameRows = m.windowHeight - 4
	if got := m.fits([]string{strings.Repeat("x", 3*m.width), "the next line"}); got != 1 {
		t.Errorf("fits = %d, want 1", got)
	}
}

func TestALineTooTallForTheBudgetStillGoesOut(t *testing.T) {
	m := sized()
	m.frameRows = m.windowHeight
	if got := m.fits([]string{strings.Repeat("x", 3*m.width)}); got != 1 {
		t.Errorf("fits = %d, want 1", got)
	}
}

func TestTheFrameNeverFillsTheWindow(t *testing.T) {
	m := sized()
	frame := m.liveRows + 2 + m.prompt.Height() + m.menu.Height() + m.queuedHeight()
	if frame >= m.windowHeight {
		t.Errorf("frame is %d rows, want room left", frame)
	}
}

func TestTheBlankRunClearPushesIsCutLikeEveryOtherBatch(t *testing.T) {
	m := sized()
	m.say(fromReader, "/clear")
	m.View()
	batches := pieces(t, m.clear())
	if len(batches) < 2 {
		t.Fatalf("printed %d batches, want at least 2", len(batches))
	}
}

func TestTheBudgetFollowsTheFrameOnScreenNotTheOnePlanned(t *testing.T) {
	m := sized()
	m.run.answer.WriteString(strings.Repeat("a streamed line\n", 40))
	m.prompt.SetValue(strings.Repeat("a typed line\n", 12))
	m.menu.Filtered = m.menu.Items
	m.layout(m.windowHeight)
	m.View()
	drawn, free := m.frameRows, m.budget()
	m.prompt.Reset()
	m.menu.Filtered = nil
	m.layout(m.windowHeight)
	if got := m.budget(); got != free {
		t.Errorf("budget = %d, want %d from drawn frame %d", got, free, drawn)
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
		if !strings.HasSuffix(name, ".go") || name == "resize.go" {
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
