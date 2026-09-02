package main

import (
	"os"
	"path/filepath"
	"testing"

	s "github.com/FacileStudio/nacelle-tui/internal/settings"
)

// TestCompactAtIsConfigurable checks the setting reaches the model through
// every layer of the precedence chain, and that the default is the one
// documented in configuration.md.
// TestCompactAtIsConfigurable tests the precedence chain for the compact
// threshold setting. It writes ~/.nacelle.yml via HOME pointing at a temp dir
// rather than trying to stub the path out, and each sub-test isolates its env
// so a preceding env doesn't leak into the file-only or default-only cases.
func TestCompactAtIsConfigurable(t *testing.T) {
	defaults := s.Defaults("")
	if *defaults.CompactAt != s.DefaultCompactAt {
		t.Fatalf("default compact_at = %d, want %d", *defaults.CompactAt, s.DefaultCompactAt)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	file := filepath.Join(home, s.ConfigFile)

	writeFile := func(t *testing.T, body string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	read := func(t *testing.T, over s.Config) s.Config {
		t.Helper()
		c, err := s.Settings("", over)
		if err != nil {
			t.Fatalf("Settings: %v", err)
		}
		return c
	}

	t.Run("default", func(t *testing.T) {
		c := read(t, s.Config{})
		if *c.CompactAt != s.DefaultCompactAt {
			t.Errorf("default compact_at = %d, want %d", *c.CompactAt, s.DefaultCompactAt)
		}
	})

	t.Run("file beats default", func(t *testing.T) {
		writeFile(t, "compact_at: 204800\n")
		c := read(t, s.Config{})
		if *c.CompactAt != 204800 {
			t.Errorf("file compact_at = %d, want 204800", *c.CompactAt)
		}
	})

	t.Run("env beats file", func(t *testing.T) {
		writeFile(t, "compact_at: 204800\n")
		t.Setenv("NACELLE_COMPACT_AT", "300000")
		c := read(t, s.Config{})
		if *c.CompactAt != 300000 {
			t.Errorf("env compact_at = %d, want 300000", *c.CompactAt)
		}
	})

	t.Run("flag beats env and file", func(t *testing.T) {
		writeFile(t, "compact_at: 204800\n")
		t.Setenv("NACELLE_COMPACT_AT", "300000")
		c := read(t, s.Config{Limits: s.Limits{CompactAt: int64Ptr(400000)}})
		if *c.CompactAt != 400000 {
			t.Errorf("flag compact_at = %d, want 400000 (flag beats env and file)", *c.CompactAt)
		}
	})

	t.Run("unmentioned key keeps default", func(t *testing.T) {
		writeFile(t, "backend: anthropic\n")
		c := read(t, s.Config{})
		if *c.CompactAt != s.DefaultCompactAt {
			t.Errorf("unmentioned compact_at = %d, want default %d", *c.CompactAt, s.DefaultCompactAt)
		}
	})

	t.Run("zero in file is honoured", func(t *testing.T) {
		writeFile(t, "compact_at: 0\n")
		c := read(t, s.Config{})
		if *c.CompactAt != 0 {
			t.Errorf("compact_at: 0 = %d, want 0 (a zero is a real value)", *c.CompactAt)
		}
	})
}

// TestCompactAtZeroDisablesCompaction confirms that a model built with a
// zero threshold never compacts, even when the transcript is enormous.
func TestCompactAtZeroDisablesCompaction(t *testing.T) {
	m := newModel(nil, "test · model", nil, 0)
	m.conversation = bigConversation()
	m.size = 5_000_000

	m.compact()

	if m.trimmed != 0 {
		t.Errorf("a zero threshold trimmed %d results; compaction should be off", m.trimmed)
	}
}

// TestCompactAtCustomThresholdCompactsAtTheCustomPoint confirms the model
// compacts at the value it was given rather than at the default.
func TestCompactAtCustomThresholdCompactsAtTheCustomPoint(t *testing.T) {
	t.Run("under threshold", func(t *testing.T) {
		spacious := newModel(nil, "test · model", nil, 200_000)
		spacious.conversation = bigConversation()
		spacious.size = 150_000
		spacious.compact()
		if spacious.trimmed != 0 {
			t.Errorf("at 150k under the 200k threshold trimmed %d, want 0", spacious.trimmed)
		}
	})

	t.Run("over threshold", func(t *testing.T) {
		lower := newModel(nil, "test · model", nil, 120_000)
		lower.conversation = bigConversation()
		lower.size = 150_000
		lower.compact()
		if lower.trimmed == 0 {
			t.Errorf("at 150k with a 120k threshold trimmed nothing; want compaction")
		}
	})
}

func int64Ptr(i int64) *int64 {
	return &i
}
