package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	s "github.com/FacileStudio/nacelle-tui/internal/settings"
)

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

func int64Ptr(i int64) *int64 {
	return &i
}
