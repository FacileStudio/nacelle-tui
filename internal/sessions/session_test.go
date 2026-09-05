package sessions

import (
	"os"
	"strings"
	"testing"
)

func TestListSessionFilesEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	files := ListSessionFiles("")
	if len(files) != 0 {
		t.Errorf("got %v, want empty", files)
	}
}

func TestLoadSessionMissingFile(t *testing.T) {
	msgs := LoadSession("/nonexistent/file.jsonl")
	if msgs != nil {
		t.Errorf("got %v, want nil", msgs)
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	log.Line(fromReader, "what is 2+2?")
	log.Line(fromModel, "4")

	files := ListSessionFiles("")
	if len(files) == 0 {
		t.Fatal("expected at least one session file")
	}

	msgs := LoadSession(files[0])
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}

	formatted := FormatSessionEntry(files[0])
	if !strings.Contains(formatted, "anthropic") || !strings.Contains(formatted, "claude-opus-5") {
		t.Errorf("formatted = %q, want backend and model", formatted)
	}
}

func TestFormatSessionEntryNonexistent(t *testing.T) {
	formatted := FormatSessionEntry("/nonexistent/test.jsonl")
	if !strings.Contains(formatted, "error reading file") {
		t.Errorf("formatted = %q, want error indicator", formatted)
	}
}

func TestFormatSessionEntryShortStarted(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/short.jsonl"
	content := `{"v":1,"started":"short","backend":"google","model":"gemini","root":"."}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	formatted := FormatSessionEntry(path)
	if !strings.Contains(formatted, "google") || !strings.Contains(formatted, "short") {
		t.Errorf("formatted = %q, want backend and short started", formatted)
	}
}

func TestListSessionFilesFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}

	files := ListSessionFiles("/nonexistent/project/dir")
	if len(files) == 0 {
		t.Fatal("expected fallback to base sessions directory")
	}
}
