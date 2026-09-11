package settings

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"
)

// The scaffold must round-trip: written as the template, read back through
// the strict decoder, and merged with the defaults it must not disturb.
func TestScaffoldRoundTripsToTheDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".nacelle.yml")

	created, err := Scaffold(path)
	if err != nil || !created {
		t.Fatalf("scaffold: created=%t err=%v", created, err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("the scaffolded file does not parse: %v", err)
	}

	resolved := Defaults("")
	resolved.mergeStrings(loaded)
	resolved.merge(loaded)

	base := Defaults("")
	want, _ := yaml.Marshal(base)
	got, _ := yaml.Marshal(resolved)
	if string(want) != string(got) {
		t.Errorf("scaffold+merge drifted from the defaults:\nbase %s\nresolved %s", want, got)
	}

	again, err := Scaffold(path)
	if err != nil || again {
		t.Errorf("scaffold rewrote an existing file: created=%t err=%v", again, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("existing file vanished: %v", err)
	}
}
