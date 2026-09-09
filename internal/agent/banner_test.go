package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountedNounPluralizes(t *testing.T) {
	cases := map[int]string{0: "0 skills", 1: "1 skill", 2: "2 skills", 17: "17 skills"}
	for n, want := range cases {
		if got := countedNoun(n, "skill"); got != want {
			t.Errorf("countedNoun(%d, \"skill\") = %q, want %q", n, got, want)
		}
	}
}

func TestBannerShowsBackendModelRootSkillsAndContextFiles(t *testing.T) {
	off := false
	got := testBanner(&answeringStub{}, asSettled(Config{Provider: Provider{Model: "claude-opus-5"}, Root: ".", Toggles: Toggles{Bash: &off}}),
		loaded{skills: []skill{{Name: "deploy"}, {Name: "filet"}}, contextFiles: 2}, connected{})

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("banner = %q, want exactly two lines", got)
	}
	if !strings.Contains(lines[0], "stub") || !strings.Contains(lines[0], "claude-opus-5") {
		t.Errorf("first line = %q, want the backend and model", lines[0])
	}
	if !strings.Contains(lines[1], "2 skills") || !strings.Contains(lines[1], "2 context files") {
		t.Errorf("second line = %q, want the skill count and the context file count", lines[1])
	}
	if !strings.Contains(lines[1], "bash off") {
		t.Errorf("second line = %q, want it to say whether the model can run commands", lines[1])
	}
}

func TestBannerSaysWhenBashIsOn(t *testing.T) {
	on := true
	got := testBanner(&answeringStub{}, asSettled(Config{Root: ".", Toggles: Toggles{Bash: &on}}), loaded{}, connected{})

	if !strings.Contains(got, "bash on") {
		t.Errorf("banner = %q, want it to say bash is on", got)
	}
}

func TestBannerResolvesRootToAnAbsolutePath(t *testing.T) {
	off := false
	got := testBanner(&answeringStub{}, asSettled(Config{Root: ".", Toggles: Toggles{Bash: &off}}), loaded{}, connected{})

	if strings.Contains(got, "\n.") || strings.HasSuffix(strings.Split(got, "\n")[1], " . ") {
		t.Errorf("banner = %q, want root resolved, not echoed as \".\"", got)
	}
}

func TestAugmentSystemCountsContextFilesAndSkills(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project preference"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	skillDir := t.TempDir()
	writeSkill(t, filepath.Join(skillDir, "deploy"), "name: deploy\ndescription: ships the app")

	config := defaults()
	config.Root = root
	config.SkillDirs = []string{skillDir}

	found := augmentSystem(&config)

	if found.contextFiles != 1 {
		t.Errorf("contextFiles = %d, want 1", found.contextFiles)
	}
	if len(found.skills) != 1 || found.skills[0].Name != "deploy" {
		t.Errorf("skills = %+v, want the one skill under -skill-dir", found.skills)
	}
}

func TestTheBannerNamesSearchOnlyWhenAnInstanceIsSet(t *testing.T) {
	without := testBanner(&answeringStub{}, asSettled(Config{Root: "."}), loaded{}, connected{})
	if strings.Contains(without, "search on") {
		t.Errorf("banner = %q, want no mention of search when none is configured", without)
	}

	with := testBanner(&answeringStub{}, asSettled(Config{Root: ".", Web: Web{Search: ptr("https://furet.example")}}), loaded{}, connected{})
	if !strings.Contains(with, "search on") {
		t.Errorf("banner = %q, want it to confirm search is on", with)
	}
}
