package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupDiagnosticsOnlyFiresWhenFiletIsConfigured(t *testing.T) {
	m := bareBanner()
	m.diagLoop = true
	if cmd := m.startupDiagnostics(); cmd != nil {
		t.Error("a root with no filet.yml must not arm the sweep")
	}
	m.diagLoop = false
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "filet.yml"), []byte("kind: cli\n"), 0o644)
	m.run.root = dir
	m.diagLoop = true
	if cmd := m.startupDiagnostics(); cmd == nil {
		t.Error("filet.yml under the root with the loop on must arm the sweep")
	}
}

func TestStartupDiagnosticsSaysNothingWhenEmpty(t *testing.T) {
	m := bareBanner()
	before := strings.Join(spoken(m), "\n")
	if cmd := m.recordStartupDiagnostics(""); cmd != nil {
		t.Fatal("an empty note must not be said")
	}
	if after := strings.Join(spoken(m), "\n"); after != before {
		t.Errorf("an empty note changed the transcript: %q", after)
	}
	m.recordStartupDiagnostics("filet: ran clean, no diagnostics")
	if !strings.Contains(strings.Join(spoken(m), "\n"), "filet: ran clean") {
		t.Error("the note never reached the transcript")
	}
}
