package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var muted = lipgloss.NewStyle()

// plain is a rendered diff with the colour escapes stripped, which is what a
// test checking for words rather than colours wants.
func plain(diff string) string {
	return ansi.Strip(diff)
}

func TestADiffShowsRemovalsAndAdditions(t *testing.T) {
	change := EditChange{Path: "main.go", Before: "one\ntwo\nthree\n", After: "one\nTWO\nthree\n"}
	diff := RenderDiff(change, 80, "32", muted)
	text := plain(diff)

	if !strings.Contains(text, "- two") {
		t.Errorf("diff = %q, want the removed line", text)
	}
	if !strings.Contains(text, "+ TWO") {
		t.Errorf("diff = %q, want the added line", text)
	}
	for _, landmark := range []string{"    one", "    three"} {
		if !strings.Contains(text, landmark) {
			t.Errorf("diff = %q, want %q kept as context", text, strings.TrimSpace(landmark))
		}
	}
}

func TestADiffNamesTheFileAtTheTop(t *testing.T) {
	change := EditChange{Path: "main.go", Before: "old\n", After: "new\n"}
	diff := plain(RenderDiff(change, 80, "32", muted))

	if !strings.Contains(diff, "main.go") {
		t.Errorf("diff = %q, want the changed file named in the header", diff)
	}
}

func TestADiffSitsInAFullWidthBoxWithAColouredSpine(t *testing.T) {
	change := EditChange{Path: "f", Before: "old\n", After: "new\n"}
	diff := RenderDiff(change, 40, "32", muted)

	for line := range strings.SplitSeq(strings.TrimSuffix(diff, "\n"), "\n") {
		if !strings.HasPrefix(plain(line), "▌") {
			t.Errorf("line %q lacks the left border", line)
		}
		if width := len([]rune(plain(line))); width != 40 {
			t.Errorf("line %q is %d cells, want the full 40-cell pane", line, width)
		}
	}
	if !strings.Contains(diff, "48;5;237") {
		t.Errorf("diff = %q, want the pane's shared background", diff)
	}
}

func TestADiffColoursRemovalsRedAndAdditionsGreen(t *testing.T) {
	change := EditChange{Path: "f", Before: "old\n", After: "new\n"}
	diff := RenderDiff(change, 80, "32", muted)

	if !strings.Contains(diff, "91") || !strings.Contains(diff, "48;5;52") {
		t.Errorf("diff = %q, want removals in red on a dark red ground", diff)
	}
	if !strings.Contains(diff, "92") || !strings.Contains(diff, "48;5;22") {
		t.Errorf("diff = %q, want additions in green on a dark green ground", diff)
	}
}

func TestARecapSumsAddedAndRemovedLines(t *testing.T) {
	change := EditChange{Path: "f", Before: "a\nb\nc\n", After: "a\nx\nc\nd\n"}
	diff := plain(RenderDiff(change, 80, "32", muted))

	if !strings.Contains(diff, "+2") || !strings.Contains(diff, "-1") {
		t.Errorf("recap = %q, want +2 -1 for one line changed and one added", diff)
	}
}

// TestARecapPutsBothCountsOnTheBlockBackground also checks the gap between the
// two figures: each figure ends in a full reset, so the space joining them must
// be styled onto the block background itself, not left bare on the terminal
// default, which would read as a visibly different patch in the pane.
func TestARecapPutsBothCountsOnTheBlockBackground(t *testing.T) {
	change := EditChange{Path: "f", Before: "a\nb\n", After: "a\nc\n"}
	var recap string
	for line := range strings.SplitSeq(strings.TrimSuffix(RenderDiff(change, 80, "32", muted), "\n"), "\n") {
		if strings.Contains(plain(line), "-1") {
			recap = line
		}
	}
	if !strings.Contains(recap, "92;48;5;237") || !strings.Contains(recap, "91;48;5;237") {
		t.Errorf("recap = %q, want both counts on the block background", recap)
	}
	if !strings.Contains(recap, "48;5;237m \x1b[m") {
		t.Errorf("recap = %q, want the gap between the counts on the block background", recap)
	}
}

func TestACreatedFileIsAllAdditions(t *testing.T) {
	change := EditChange{Path: "new.go", Before: "", After: "package main\n"}
	diff := RenderDiff(change, 80, "32", muted)
	text := plain(diff)

	if strings.Contains(diff, "48;5;52") {
		t.Errorf("diff = %q, want no red ground for a new file", diff)
	}
	if !strings.Contains(text, "+ package main") {
		t.Errorf("diff = %q, want the written line as an addition", text)
	}
	if strings.Contains(text, "- ") {
		t.Errorf("diff = %q, want no removals for a new file", text)
	}
}

func TestAnUnchangedFileRendersNothing(t *testing.T) {
	change := EditChange{Path: "same.go", Before: "a\nb\n", After: "a\nb\n"}
	if diff := RenderDiff(change, 80, "32", muted); diff != "" {
		t.Errorf("diff = %q, want nothing for identical contents", diff)
	}
}

func TestADiffIsCutToTheWindowWithoutWrapping(t *testing.T) {
	long := strings.Repeat("x", 200)
	change := EditChange{Path: "f", Before: long + "\n", After: long + "\nadded\n"}
	diff := RenderDiff(change, 40, "32", muted)

	for line := range strings.SplitSeq(strings.TrimSuffix(diff, "\n"), "\n") {
		if width := len([]rune(plain(line))); width > 40 {
			t.Errorf("line %q is %d cells, wider than the 40-cell pane", line, width)
		}
	}
}

func TestAHugeRewriteFallsBackToOneReplacementBlock(t *testing.T) {
	before := strings.Repeat("old line\n", 3000)
	after := strings.Repeat("new line\n", 3000)
	change := EditChange{Path: "f", Before: before, After: after}

	diff := RenderDiff(change, 80, "32", muted)
	if got := strings.Count(plain(diff), "+ new line"); got == 0 {
		t.Error("diff shows no additions for a wholesale rewrite")
	}
	if lines := len(strings.Split(plain(diff), "\n")); lines > shownDiffLines+8 {
		t.Errorf("diff renders %d lines, want it capped near %d", lines, shownDiffLines)
	}
}

func TestCountChangeCountsAllLinesNotJustShown(t *testing.T) {
	before := strings.Repeat("r\n", 100)
	after := strings.Repeat("a\n", 200)
	change := EditChange{Path: "f", Before: before, After: after}

	added, removed := CountChange(change)
	if added != 200 || removed != 100 {
		t.Errorf("count = %d,%d want 200,100", added, removed)
	}
}

func TestCaptureEditReadsBothKindsOfEditingCall(t *testing.T) {
	change, ok := CaptureEdit(".", "edit_file", `{"path":"view.go","old":"a","new":"b"}`)
	if !ok {
		t.Fatal("edit_file was not captured")
	}
	if change.Path != "view.go" || change.Before != "a" || change.After != "b" {
		t.Errorf("change = %+v, want path/old/new read from the input", change)
	}

	change, ok = CaptureEdit(".", "write_file", `{"path":"fresh.txt","content":"hello"}`)
	if !ok {
		t.Fatal("write_file was not captured")
	}
	if change.After != "hello" || change.Before != "" {
		t.Errorf("change = %+v, want the content argument as after and nothing as before", change)
	}
}

func TestCaptureEditReadsThePriorContentsOfAWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "there.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	change, ok := CaptureEdit(root, "write_file", `{"path":"there.txt","content":"after"}`)
	if !ok {
		t.Fatal("write_file was not captured")
	}
	if change.Before != "before\n" {
		t.Errorf("before = %q, want what the file held", change.Before)
	}

	change, _ = CaptureEdit(root, "write_file", `{"path":"absent.txt","content":"after"}`)
	if change.Before != "" {
		t.Errorf("before = %q, want empty for a file being created", change.Before)
	}
}

func TestCaptureEditRefusesWhatItCannotRenderHonestly(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		input string
	}{
		{"not an editing tool", "run_command", `{"command":"ls"}`},
		{"an unknown tool", "some_mcp_tool", `{"path":"a","content":"b"}`},
		{"duplicate keys", "edit_file", `{"path":"a","path":"b","old":"c","new":"d"}`},
		{"missing path", "edit_file", `{"old":"a","new":"b"}`},
		{"missing old", "edit_file", `{"path":"a","new":"b"}`},
		{"missing content", "write_file", `{"path":"a"}`},
		{"input is not an object", "edit_file", `"a"`},
	}
	for _, c := range cases {
		if _, ok := CaptureEdit(".", c.tool, c.input); ok {
			t.Errorf("%s was captured: %s(%s)", c.name, c.tool, c.input)
		}
	}
}
