package diff

import (
	"strings"
)

// extractEditPath parses a shell command looking for known in-place file editing
// commands (sed -i, awk -i inplace, perl -i) and extracts the target file path.
func extractEditPath(cmd string) (string, bool) {
	after, matched := matchInplaceMarker(cmd)
	if !matched {
		return "", false
	}
	return firstPathArg(after)
}

func matchInplaceMarker(cmd string) (string, bool) {
	for _, m := range []string{"sed -i", "awk -i", "perl -i"} {
		if _, after, matched := strings.Cut(cmd, m); matched {
			return after, true
		}
	}
	return "", false
}

func firstPathArg(after string) (string, bool) {
	fields := strings.Fields(after)
	for i, f := range fields {
		if i == 0 && isFirstArgExtension(f) {
			continue
		}
		if !isPathToken(f) {
			continue
		}
		return strings.Trim(f, `"'`), true
	}
	return "", false
}

func isFirstArgExtension(f string) bool {
	if f == `''` || f == `""` {
		return true
	}
	return !strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "'") && !strings.HasPrefix(f, `"`)
}

func isPathToken(f string) bool {
	if strings.HasPrefix(f, "-") || f == "inplace" {
		return false
	}
	if len(f) > 2 && (strings.HasPrefix(f, "'") || strings.HasPrefix(f, `"`)) {
		return false
	}
	return true
}
