// Package diff formats and renders file diffs.
package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// editTools are the local tools whose result changes a file's contents, and
// therefore the only calls a diff is drawn for. Everything else — search,
// commands, MCP tools under any name — renders exactly as it did before.
var editTools = map[string]bool{
	"edit_file":   true,
	"write_file":  true,
	"run_command": true,
}

// IsEditTool reports whether a tool's result is drawn as a boxed change (a
// file diff, or a run_command's output). Every other tool prints its ordinary
// one-line report.
func IsEditTool(name string) bool {
	return editTools[name]
}

// contextLines is how many unchanged lines surround each block of changes,
// the same neighbourhood every git reader defaults to.
const contextLines = 3

// shownDiffLines caps how much of one diff reaches the screen. A diff is a
// summary a person glances at, not a second copy of the file, and a rewrite
// of a ten-thousand-line file would otherwise print ten thousand lines into
// scrollback that can never be redrawn.
const shownDiffLines = 400

// EditChange is what one editing call did to one file: its path relative to
// root, and the contents either side of it.
//
// An edit_file carries both sides in its own input — the exact text it
// replaces, and the text replacing it — so nothing has to be read from disk.
// A write_file carries only the new contents; its before side is whatever the
// file held when the call was seen, empty for a file being created.
type EditChange struct {
	Path   string
	Before string
	After  string
}

// CaptureEdit turns one tool call into the change it will make, and says
// whether it is one worth drawing a diff for.
// CaptureEdit turns a tool call's raw input into the before-and-after text of
// the file it touches, ready for diffing. Calls that cannot be parsed, or tools
// that do not edit files, return false and produce no diff.
func CaptureEdit(root, name, input string) (EditChange, bool) {
	if !editTools[name] {
		return EditChange{}, false
	}
	fields, err := strictObject([]byte(input))
	if err != nil {
		return EditChange{}, false
	}
	return changeFrom(root, name, fields)
}

// changeFrom reads the arguments one editing call diffs over, tool by tool.
func changeFrom(root, name string, fields map[string]json.RawMessage) (EditChange, bool) {
	switch name {
	case "edit_file":
		return editFileChange(fields)
	case "write_file":
		return writeFileChange(root, fields)
	case "run_command":
		return runCommandChange(root, fields)
	default:
		return EditChange{}, false
	}
}

func editFileChange(fields map[string]json.RawMessage) (EditChange, bool) {
	path, ok1 := fieldString(fields, "path")
	before, ok2 := fieldString(fields, "old")
	after, ok3 := fieldString(fields, "new")
	if !ok1 || !ok2 || !ok3 {
		return EditChange{}, false
	}
	return EditChange{Path: path, Before: before, After: after}, true
}

func writeFileChange(root string, fields map[string]json.RawMessage) (EditChange, bool) {
	path, ok1 := fieldString(fields, "path")
	after, ok2 := fieldString(fields, "content")
	if !ok1 || !ok2 {
		return EditChange{}, false
	}
	return EditChange{Path: path, Before: PriorContents(root, path), After: after}, true
}

func runCommandChange(root string, fields map[string]json.RawMessage) (EditChange, bool) {
	cmd, ok := fieldString(fields, "command")
	if !ok {
		return EditChange{}, false
	}
	path, ok := extractEditPath(cmd)
	if !ok {
		return EditChange{}, false
	}
	return EditChange{Path: path, Before: PriorContents(root, path)}, true
}

// fieldString reads one string argument out of already-parsed input.
func fieldString(fields map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := fields[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// PriorContents is what a file held before a write_file replaced it, and ""
// when there was nothing to hold — including every reason a read can fail,
// because a file about to be overwritten is not made less writable by an
// unreadable permission bit.
func PriorContents(root, path string) string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return ""
	}
	return string(raw)
}
