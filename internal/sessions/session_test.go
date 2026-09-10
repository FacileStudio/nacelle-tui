package sessions

import (
	"os"
	"path/filepath"
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

func TestResolveSessionByIdAndPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	path := log.Path()

	if resolved := ResolveSession(path); resolved != path {
		t.Errorf("resolved %q by path, want %q", resolved, path)
	}
	if resolved := ResolveSession(filepath.Base(path)); resolved != path {
		t.Errorf("resolved %q by id, want %q", resolved, path)
	}
	if resolved := ResolveSession(filepath.Base(path) + ".jsonl"); resolved != path {
		t.Errorf("resolved %q by id with suffix, want %q", resolved, path)
	}
	if resolved := ResolveSession("no-such-session"); resolved != "" {
		t.Errorf("resolved %q for a missing id, want empty", resolved)
	}
	if resolved := ResolveSession(""); resolved != "" {
		t.Errorf("resolved %q for empty, want empty", resolved)
	}
}

func TestRestoreAtLaunch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	convo, name, err := RestoreAtLaunch("", "/repo", false)
	if convo != nil || name != "" || err != "" {
		t.Errorf("no-op restore = %v/%q/%q, want nil/empty/empty", convo, name, err)
	}

	convo, name, err = RestoreAtLaunch("no-such-session", "/repo", false)
	if convo != nil || name != "" || err == "" {
		t.Errorf("missing resume = %v/%q/%q, want nil and an error", convo, name, err)
	}

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	log.Line(fromReader, "hi")

	convo, name, err = RestoreAtLaunch(filepath.Base(log.Path()), "/repo", false)
	if convo == nil || len(convo) != 1 || name == "" || err != "" {
		t.Errorf("explicit resume = %v/%q/%q, want one message and no error", convo, name, err)
	}

	convo, name, err = RestoreAtLaunch("", "/repo", true)
	if convo == nil || len(convo) != 1 || name == "" || err != "" {
		t.Errorf("auto resume = %v/%q/%q, want the newest session and no error", convo, name, err)
	}
}
