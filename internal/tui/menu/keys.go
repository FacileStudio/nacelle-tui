package menu

import "strings"

// CommandWord extracts the command word starting with / up to whitespace.
func CommandWord(value string) string {
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	rest := value[1:]
	if i := strings.IndexAny(rest, " \t\n"); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}

// AnyCommand finds the first /command anywhere in value.
func AnyCommand(value string) string {
	idx := strings.Index(value, "/")
	if idx < 0 {
		return ""
	}
	rest := value[idx+1:]
	if i := strings.IndexAny(rest, " \t\n"); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}

// InsertPick returns the picked command with a trailing space.
func InsertPick(value, pick string) string {
	return pick + " "
}

// ReplaceCommand replaces the first /command in value with pick.
func ReplaceCommand(value, pick string) string {
	idx := strings.Index(value, "/")
	if idx < 0 {
		return value
	}
	rest := value[idx+1:]
	end := strings.IndexAny(rest, " \t\n")
	if end < 0 {
		end = len(rest)
	}
	return value[:idx] + pick + " " + rest[end:]
}
