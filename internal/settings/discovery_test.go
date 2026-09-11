package settings

import "testing"

func TestDiscoveryCanBeTurnedOffByTheFile(t *testing.T) {
	written(t, "discovery:\n  skills: false\n  project_context: false\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Skills {
		t.Error("skills = true, want the file's false to survive a default that had it on")
	}
	if *config.ProjectContext {
		t.Error("project context = true, want the file's false to survive a default that had it on")
	}
}
