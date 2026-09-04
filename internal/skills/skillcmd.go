package skills

import (
	"os"
	"sort"
)

// BySkillName keys every loaded skill by its own name.
func BySkillName(skills []Skill) map[string]Skill {
	byName := make(map[string]Skill, len(skills))
	for _, s := range skills {
		if _, exists := byName[s.Name]; !exists {
			byName[s.Name] = s
		}
	}
	return byName
}

// SkillCommandNames lists every loaded skill as a "/skill:name" suggestion.
func SkillCommandNames(skills map[string]Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, "/skill:"+name)
	}
	sort.Strings(names)
	return names
}

// SkillPrompt builds what /skill:name sends.
func SkillPrompt(s Skill, args string) (string, error) {
	body, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	text := string(body)
	if args != "" {
		text += "\n\nUser: " + args
	}
	return text, nil
}
