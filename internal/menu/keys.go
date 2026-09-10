package menu

import "strings"

// AnyCommand finds the first /command anywhere in value.
func AnyCommand(value string) string {
	_, rest, found := strings.Cut(value, "/")
	if !found {
		return ""
	}
	if i := strings.IndexAny(rest, " \t\n"); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}

// ReplaceCommand replaces the first /command in value with pick.
func ReplaceCommand(value, pick string) string {
	before, rest, found := strings.Cut(value, "/")
	if !found {
		return value
	}
	end := strings.IndexAny(rest, " \t\n")
	if end < 0 {
		end = len(rest)
	}
	return before + pick + " " + rest[end:]
}
