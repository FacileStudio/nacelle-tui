package menu

import "strings"

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
