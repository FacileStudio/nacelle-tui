package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// plain is a rendered diff with the colour escapes stripped, which is what a
// test checking for words rather than colours wants.
func plain(diff string) string {
	return ansi.Strip(diff)
}

func TestADiffShowsRemovalsAndAdditions(t *testing.T) {
	change := editChange{path: "main.go", before: "one\ntwo\nthree\n", after: "one\nTWO\nthree\n"}
	diff := renderDiff(change, 80)
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

func TestADiffColoursRemovalsRedAndAdditionsGreen(t *testing.T) {
	change := editChange{path: "f", before: "old\n", after: "new\n"}
	diff := renderDiff(change, 80)

	if !strings.Contains(diff, diffRemoved.Render("  - old")) {
		t.Errorf("diff = %q, want removals in ANSI red", diff)
	}
	if !strings.Contains(diff, diffAdded.Render("  + new")) {
		t.Errorf("diff = %q, want additions in ANSI green", diff)
	}
}

func TestACreatedFileIsAllAdditions(t *testing.T) {
	change := editChange{path: "new.go", before: "", after: "package main\n"}
	diff := plain(renderDiff(change, 80))

	if strings.Contains(diff, "- ") {
		t.Errorf("diff = %q, want no removals for a new file", diff)
	}
	if !strings.Contains(diff, "+ package main") {
		t.Errorf("diff = %q, want the written line as an addition", diff)
	}
}

func TestAnUnchangedFileRendersNothing(t *testing.T) {
	change := editChange{path: "same.go", before: "a\nb\n", after: "a\nb\n"}
	if diff := renderDiff(change, 80); diff != "" {
		t.Errorf("diff = %q, want nothing for identical contents", diff)
	}
}

func TestADiffIsCutToTheWindowWithoutWrapping(t *testing.T) {
	long := strings.Repeat("x", 200)
	change := editChange{path: "f", before: long + "\n", after: long + "\nadded\n"}
	diff := renderDiff(change, 40)

	for _, line := range strings.Split(diff, "\n") {
		if width := len([]rune(plain(line))); width > 41 {
			t.Errorf("line %q is %d cells, wider than the 40-cell window plus its prefix", line, width)
		}
	}
}

func TestAHugeRewriteFallsBackToOneReplacementBlock(t *testing.T) {
	before := strings.Repeat("old line\n", 3000)
	after := strings.Repeat("new line\n", 3000)
	change := editChange{path: "f", before: before, after: after}

	diff := renderDiff(change, 80)
	if got := strings.Count(plain(diff), "+ new line"); got == 0 {
		t.Error("diff shows no additions for a wholesale rewrite")
	}
	if lines := len(strings.Split(plain(diff), "\n")); lines > shownDiffLines+2 {
		t.Errorf("diff renders %d lines, want it capped near %d", lines, shownDiffLines)
	}
}

func TestCaptureEditReadsBothKindsOfEditingCall(t *testing.T) {
	change, ok := captureEdit(".", "edit_file", `{"path":"view.go","old":"a","new":"b"}`)
	if !ok {
		t.Fatal("edit_file was not captured")
	}
	if change.path != "view.go" || change.before != "a" || change.after != "b" {
		t.Errorf("change = %+v, want path/old/new read from the input", change)
	}

	change, ok = captureEdit(".", "write_file", `{"path":"fresh.txt","content":"hello"}`)
	if !ok {
		t.Fatal("write_file was not captured")
	}
	if change.after != "hello" || change.before != "" {
		t.Errorf("change = %+v, want the content argument as after and nothing as before", change)
	}
}

func TestCaptureEditReadsThePriorContentsOfAWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "there.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	change, ok := captureEdit(root, "write_file", `{"path":"there.txt","content":"after"}`)
	if !ok {
		t.Fatal("write_file was not captured")
	}
	if change.before != "before\n" {
		t.Errorf("before = %q, want what the file held", change.before)
	}

	change, _ = captureEdit(root, "write_file", `{"path":"absent.txt","content":"after"}`)
	if change.before != "" {
		t.Errorf("before = %q, want empty for a file being created", change.before)
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
		if _, ok := captureEdit(".", c.tool, c.input); ok {
			t.Errorf("%s was captured: %s(%s)", c.name, c.tool, c.input)
		}
	}
}
