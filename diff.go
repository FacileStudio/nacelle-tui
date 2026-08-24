package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
)

// editTools are the local tools whose result changes a file's contents, and
// therefore the only calls a diff is drawn for. Everything else — search,
// commands, MCP tools under any name — renders exactly as it did before.
var editTools = map[string]bool{
	"edit_file":  true,
	"write_file": true,
}

// contextLines is how many unchanged lines surround each block of changes,
// the same neighbourhood every git reader defaults to.
const contextLines = 3

// shownDiffLines caps how much of one diff reaches the screen. A diff is a
// summary a person glances at, not a second copy of the file, and a rewrite
// of a ten-thousand-line file would otherwise print ten thousand lines into
// scrollback that can never be redrawn.
const shownDiffLines = 400

// The added and removed sides are the terminal's own green and red, because a
// diff's two colours are the two every scheme already has an opinion about. An
// ANSI index needs no help following a light terminal: the scheme is what
// resolves it, and it resolves it again the moment the scheme changes.
//
// There is no third var for the unchanged lines between them. Those are muted
// text, the same as the queued rows and the counts under the status line, and
// the whole point of routing every grey through palette.muted is that the grey
// is decided once — by the background the terminal reported, not by whoever
// last typed a colour into a package var. It lived here as a fixed ANSI 8 and
// was the one style in this program that could not follow the background,
// which is exactly the case a constant grey gets wrong: 8 is a near-black on
// the dark schemes that move it, and a fixed mid-grey is thin on white. The
// style arrives as an argument now — see renderDiff.
var (
	diffAdded   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(2))
	diffRemoved = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1))
)

// editChange is what one editing call did to one file: its path relative to
// root, and the contents either side of it.
//
// An edit_file carries both sides in its own input — the exact text it
// replaces, and the text replacing it — so nothing has to be read from disk.
// A write_file carries only the new contents; its before side is whatever the
// file held when the call was seen, empty for a file being created.
type editChange struct {
	path   string
	before string
	after  string
}

// captureEdit turns one tool call into the change it will make, and says
// whether it is one worth drawing a diff for.
//
// Anything that is not a known editing tool, or whose input cannot be read
// cleanly, is answered with false rather than guessed at: a diff of the wrong
// two texts is worse than no diff. A duplicate key is refused for the reason
// toolline refuses one — the value shown and the value run must be the same
// value — and here the gate may be off, so refusal means silence rather than
// a denied call, which is still the honest rendering of an unreadable one.
func captureEdit(root, name, input string) (editChange, bool) {
	if !editTools[name] {
		return editChange{}, false
	}
	fields, err := strictObject([]byte(input))
	if err != nil {
		return editChange{}, false
	}
	return changeFrom(root, name, fields)
}

// changeFrom reads the arguments one editing call diffs over, tool by tool.
func changeFrom(root, name string, fields map[string]json.RawMessage) (editChange, bool) {
	path, ok := fieldString(fields, "path")
	if !ok {
		return editChange{}, false
	}
	change := editChange{path: path}

	switch name {
	case "edit_file":
		before, ok := fieldString(fields, "old")
		if !ok {
			return editChange{}, false
		}
		after, ok := fieldString(fields, "new")
		if !ok {
			return editChange{}, false
		}
		change.before, change.after = before, after
	case "write_file":
		after, ok := fieldString(fields, "content")
		if !ok {
			return editChange{}, false
		}
		change.after = after
		change.before = priorContents(root, path)
	default:
		return editChange{}, false
	}
	return change, true
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

// priorContents is what a file held before a write_file replaced it, and ""
// when there was nothing to hold — including every reason a read can fail,
// because a file about to be overwritten is not made less writable by an
// unreadable permission bit.
func priorContents(root, path string) string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return ""
	}
	return string(raw)
}

// renderDiff draws one change the way git would: removals in the terminal's
// red, additions in its green, and a few unchanged lines around each block so
// the eye can find where in the file it is looking.
//
// The colours are ANSI indices rather than fixed values, so they follow
// whatever scheme the terminal itself uses. Nothing worth showing — a call
// whose input could not be parsed, a change that touches no line — renders as
// empty, and the caller simply says the ordinary one-line report it always
// has.
func renderDiff(change editChange, width int, muted lipgloss.Style) string {
	if change.path == "" || change.before == change.after {
		return ""
	}
	blocks := hunks(diffOps(splitLines(change.before), splitLines(change.after)), contextLines)
	if len(blocks) == 0 {
		return ""
	}

	var out strings.Builder
	shown := 0
	for i, block := range blocks {
		if i > 0 {
			out.WriteString(muted.Render("  …") + "\n")
			shown++
		}
		var cut bool
		shown, cut = renderBlock(&out, block, width, shown, muted)
		if cut {
			break
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

// renderBlock writes one hunk's lines, stopping at the display cap with a
// marker saying the diff was cut rather than pretending it wasn't. It
// returns how many lines are through and whether the cap was reached.
func renderBlock(out *strings.Builder, block []diffOp, width, shown int, muted lipgloss.Style) (int, bool) {
	for _, op := range block {
		if shown >= shownDiffLines {
			out.WriteString(muted.Render("  … more"))
			return shown, true
		}
		out.WriteString(renderOp(op, width, muted))
		shown++
	}
	return shown, false
}

// renderOp is one diff line as styled text, indented past the ⏺ that names
// the call above it and cut to the window so nothing wraps.
func renderOp(op diffOp, width int, muted lipgloss.Style) string {
	style, prefix := muted, "    "
	switch op.kind {
	case '-':
		style, prefix = diffRemoved, "  - "
	case '+':
		style, prefix = diffAdded, "  + "
	}
	return style.Render(prefix+truncate(op.text, max(width-lipgloss.Width(prefix), 1))) + "\n"
}

// splitLines breaks file contents into diffable lines, dropping a single
// trailing newline — the marker of a well-formed text file, not a line of it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}
