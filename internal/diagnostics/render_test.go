package diagnostics

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func manyFindings(n int, msg string) []finding {
	fs := make([]finding, n)
	for i := range fs {
		fs[i] = finding{loc: fmt.Sprintf("a.go:%d:%d", i+1, i+1), rule: "r", msg: fmt.Sprintf("%s %d", msg, i)}
	}
	return fs
}

func TestRenderKeepsSmallListsInline(t *testing.T) {
	fs := []finding{
		{loc: "a.go:1:1", rule: "r", msg: "one"},
		{loc: "a.go:2:2", rule: "r", msg: "two"},
	}
	if want := "a.go:1:1: one\na.go:2:2: two"; render(fs) != want {
		t.Fatalf("render = %q, want %q", render(fs), want)
	}
}

func TestRenderKeepsTenFindingsWithoutSpilling(t *testing.T) {
	got := render(manyFindings(10, "boom"))
	if strings.Contains(got, "full list") {
		t.Fatalf("render spilled an in-cap list: %q", got)
	}
	if lines := strings.Split(got, "\n"); len(lines) != 10 {
		t.Fatalf("render gave %d lines, want 10", len(lines))
	}
}

func TestRenderSpillsWhenFindingsExceedTheCap(t *testing.T) {
	fs := manyFindings(12, "boom")
	got := render(fs)
	lines := strings.Split(got, "\n")
	if len(lines) != 10 {
		t.Fatalf("render gave %d lines, want 10", len(lines))
	}
	if !strings.HasPrefix(lines[9], "filet: 12 diagnostics; full list: ") {
		t.Fatalf("pointer line = %q", lines[9])
	}
	name := strings.TrimPrefix(lines[9], "filet: 12 diagnostics; full list: ")
	t.Cleanup(func() { os.Remove(name) })
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read spill: %v", err)
	}
	if want := strings.Join(linesOf(fs), "\n"); string(raw) != want {
		t.Fatalf("spill does not carry every finding:\n%s", raw)
	}
}

func TestRenderSpillsWhenTextExceedsTheByteCap(t *testing.T) {
	fs := []finding{
		{loc: "a.go:1:1", rule: "r", msg: strings.Repeat("x", maxTextBytes)},
		{loc: "a.go:2:2", rule: "r", msg: "small"},
	}
	got := render(fs)
	if lines := strings.Split(got, "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "filet: 2 diagnostics; full list: ") {
		t.Fatalf("render = %q, want a single pointer line", got)
	}
	name := strings.TrimPrefix(got, "filet: 2 diagnostics; full list: ")
	t.Cleanup(func() { os.Remove(name) })
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read spill: %v", err)
	}
	if want := strings.Join(linesOf(fs), "\n"); string(raw) != want {
		t.Fatalf("spill does not carry every finding:\n%s", raw)
	}
}
