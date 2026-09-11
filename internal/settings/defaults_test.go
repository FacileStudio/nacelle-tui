package settings

import (
	"slices"
	"testing"
)

func defaults() Config {
	return Defaults("")
}

// Unlike Bash, both default on: neither costs anything to ask for when there
// is nothing to find — no CLAUDE.md or AGENTS.md anywhere above root, no
// ~/.agents/skills — so a machine without either is no worse off, and a
// machine with them benefits without a flag to discover first. The precedence
// chain itself is already proven generic by the Bash and Thinking tests above;
// this only has to prove these two default to the right value.
func TestProjectContextAndSkillsDefaultOn(t *testing.T) {
	fallback := defaults()
	if !*fallback.ProjectContext {
		t.Error("project context = false, want it on by default")
	}
	if !*fallback.Skills {
		t.Error("skills = false, want it on by default")
	}
}

// Diffs is a display toggle with no cost when nothing edited a file, so it
// defaults on like the discovery toggles above rather than off like bash.
// This proves the default and that a layer can still turn it off — which is
// how today's one-line rendering is restored.
func TestDiffsDefaultOnAndTurnableOff(t *testing.T) {
	fallback := defaults()
	if !*fallback.Diffs {
		t.Error("diffs = false, want it on by default")
	}

	written(t, "tools:\n  diffs: false\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Diffs {
		t.Error("diffs = true, want the file's false to win")
	}
}

// Resume is a string that defaults empty and only names a session when a
// layer says so. This proves the default and that the `resume:` key decodes
// without tripping KnownFields(true), so a config that carries it keeps
// loading rather than becoming a startup error.
func TestResumeDefaultsEmptyAndComesFromTheFile(t *testing.T) {
	fallback := defaults()
	if *fallback.Resume != "" {
		t.Errorf("resume = %q, want the empty default", *fallback.Resume)
	}
	written(t, "resume: 2026-09-10T14-000Z.jsonl\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings with resume: %v", err)
	}
	if *config.Resume != "2026-09-10T14-000Z.jsonl" {
		t.Errorf("resume = %q, want the file's session id", *config.Resume)
	}
}

// Continue is a UI *bool like the group_tools and show_thinking toggles. It
// was silently dropped by mergeUI for a long time — a layer that set it never
// reached the resolved config — so this pins the merge that keeps auto-resume
// switchable.
func TestContinueFromTheFile(t *testing.T) {
	written(t, "continue: true\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings with continue: %v", err)
	}
	if !*config.Continue {
		t.Error("continue = false, want the file's true to win")
	}
}

// JSON rides the same UI merge as continue, and this pins that a file layer
// saying cron_list_json: true reaches the resolved config instead of being dropped by
// mergeUI.
func TestJSONFromTheFile(t *testing.T) {
	written(t, "ui:\n  cron_list_json: true\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings with json: %v", err)
	}
	if !*config.JSON {
		t.Error("json = false, want the file's true to win")
	}
}

// SkillDirs is the one setting that is a slice rather than a string or a
// *bool, so it needed its own line in merge() — this proves that line
// actually runs, the same way TestTheFileBeatsTheDefaults proves it for a
// plain string.
func TestSkillDirsComesFromTheFile(t *testing.T) {
	written(t, "sources:\n  skill_dirs:\n    - /a/skills\n    - /b/skills\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if want := []string{"/a/skills", "/b/skills"}; !slices.Equal(config.SkillDirs, want) {
		t.Errorf("skill dirs = %v, want %v", config.SkillDirs, want)
	}
}

// NACELLE_SKILL_DIRS is colon-separated, the same convention PATH itself
// uses for a list of directories, and it has to beat the file without
// erasing an unrelated setting the file made.
func TestSkillDirsFromTheEnvironmentAreColonSeparatedAndBeatTheFile(t *testing.T) {
	written(t, "sources:\n  skill_dirs:\n    - /from/the/file\n")
	t.Setenv(EnvPrefix+"SKILL_DIRS", "/a/skills:/b/skills")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if want := []string{"/a/skills", "/b/skills"}; !slices.Equal(config.SkillDirs, want) {
		t.Errorf("skill dirs = %v, want the environment's list", config.SkillDirs)
	}
}
