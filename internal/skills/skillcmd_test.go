package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBySkillNameKeepsTheFirstOnACollision(t *testing.T) {
	byName := BySkillName([]Skill{
		{Name: "deploy", Path: "/first/SKILL.md"},
		{Name: "deploy", Path: "/second/SKILL.md"},
	})

	if got := byName["deploy"].Path; got != "/first/SKILL.md" {
		t.Errorf("path = %q, want the first skill found kept", got)
	}
}

func TestSkillCommandNamesListsEverySkillSlashPrefixed(t *testing.T) {
	names := SkillCommandNames(BySkillName([]Skill{{Name: "a"}, {Name: "b"}}))

	if want := []string{"/skill:a", "/skill:b"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestSkillPromptSendsTheSkillsFullBody(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "Name: deploy\ndescription: ships the app")
	s := Skill{Name: "deploy", Path: filepath.Join(dir, "SKILL.md")}

	got, err := SkillPrompt(s, "")
	if err != nil {
		t.Fatalf("skillPrompt: %v", err)
	}
	if !strings.Contains(got, "Do the thing.") {
		t.Errorf("prompt = %q, want the skill body's own instructions", got)
	}
	if strings.Contains(got, "User:") {
		t.Errorf("prompt = %q, want no User: line when nothing followed the name", got)
	}
}

func TestSkillPromptAppendsArgsAsAUserLine(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "Name: deploy\ndescription: ships the app")
	s := Skill{Name: "deploy", Path: filepath.Join(dir, "SKILL.md")}

	got, err := SkillPrompt(s, "to staging")
	if err != nil {
		t.Fatalf("skillPrompt: %v", err)
	}
	if !strings.HasSuffix(got, "User: to staging") {
		t.Errorf("prompt = %q, want it to end with the args as a User: line", got)
	}
}

func TestSkillPromptFailsOnAMissingFile(t *testing.T) {
	if _, err := SkillPrompt(Skill{Path: filepath.Join(t.TempDir(), "gone", "SKILL.md")}, ""); err == nil {
		t.Error("skillPrompt succeeded reading a file that was never there")
	}
}
