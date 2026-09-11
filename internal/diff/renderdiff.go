package diff

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FacileStudio/nacelle-tui/internal/toolview"
)

var (
	// addedStyle tints a changed line's text green on a dark green backdrop.
	addedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Background(lipgloss.Color("22"))
	// removedStyle tints a changed line's text red on a dark red backdrop.
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("52"))
	// addFg and remFg colour the recap's +x / -y figures on the block
	// background. Each carries that background itself because a rendered
	// fragment ends in a full reset, which would otherwise cancel the
	// surrounding cell's background before the second figure draws.
	addFg = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Background(lipgloss.Color(toolview.BlockBg))
	remFg = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color(toolview.BlockBg))
)

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return ansi.Truncate(s, limit, "…")
}

// block lays a muted line over the box background, so context, the header and
// the recap share the pane with the changed lines instead of floating on it.
func block(muted lipgloss.Style) lipgloss.Style {
	return muted.Background(lipgloss.Color(toolview.BlockBg))
}

// splitLines breaks file contents into diffable lines, dropping a single
// trailing newline — the marker of a well-formed text file, not a line of it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// cell renders one content line padded to the full pane width, so a box row
// spans width-1 columns and the left border is the one column it adds.
func cell(base lipgloss.Style, text string, content int) string {
	return base.Width(content).Render(text)
}

// CountChange reports how many lines a change adds and removes, whatever the
// display cap on the diff itself. The recap under a block states the whole
// change, not the window it was shown in.
func CountChange(change EditChange) (added, removed int) {
	if change.Path == "" || change.Before == change.After {
		return 0, 0
	}
	for _, op := range diffOps(splitLines(change.Before), splitLines(change.After)) {
		switch op.kind {
		case '+':
			added++
		case '-':
			removed++
		}
	}
	return added, removed
}

// RenderDiff draws one change as a full-width box: a coloured left border, a
// header naming the file, the additions in green and the removals in red each
// on their own tinted background, and a recap footer of "+x -y". borderColor
// is the caller's verdict on the edit — green or red when it finished, the
// tool's own colour while it is still running.
func RenderDiff(change EditChange, width int, borderColor string, muted lipgloss.Style) string {
	if change.Path == "" || change.Before == change.After {
		return ""
	}
	ops := diffOps(splitLines(change.Before), splitLines(change.After))
	blocks := hunks(ops, contextLines)
	if len(blocks) == 0 {
		return ""
	}
	content := max(width-1, 10)

	rows := make([]string, 0, shownDiffLines+2)
	rows = append(rows, cell(block(muted), "  "+change.Path, content))
	shown := 0
	for i, blk := range blocks {
		if i > 0 {
			rows = append(rows, cell(block(muted), "  …", content))
			shown++
		}
		var cut bool
		rows, shown, cut = renderBlock(rows, blk, content, shown, muted)
		if cut {
			break
		}
	}
	rows = append(rows, recap(change, muted, content))
	return toolview.Box(rows, borderColor)
}

// recap is the box's footer: "+x -y", additions counted in green and removals
// in red, sharing the block background with the header. Each figure fragment
// ends in a full reset, so the space joining them must carry the block
// background itself — a literal space would render bare on the terminal's
// default background, a visibly different patch in the pane.
func recap(change EditChange, muted lipgloss.Style, content int) string {
	added, removed := CountChange(change)
	var parts []string
	if added > 0 {
		parts = append(parts, addFg.Render("+"+fmt.Sprintf("%d", added)))
	}
	if removed > 0 {
		parts = append(parts, remFg.Render("-"+fmt.Sprintf("%d", removed)))
	}
	sep := block(muted).Render(" ")
	return cell(block(muted), "  "+strings.Join(parts, sep), content)
}

// renderBlock appends one hunk's rows to the box, stopping at the display cap
// with a marker saying the diff was cut rather than pretending it wasn't. It
// returns the grown slice, how many rows are through, and whether the cap was
// reached.
func renderBlock(rows []string, blk []diffOp, content, shown int, muted lipgloss.Style) ([]string, int, bool) {
	for _, op := range blk {
		if shown >= shownDiffLines {
			rows = append(rows, cell(block(muted), "  … more", content))
			return rows, shown, true
		}
		var row string
		switch op.kind {
		case '-':
			row = cell(removedStyle, "  - "+truncate(op.text, content-4), content)
		case '+':
			row = cell(addedStyle, "  + "+truncate(op.text, content-4), content)
		default:
			row = cell(block(muted), "    "+truncate(op.text, content-5), content)
		}
		rows = append(rows, row)
		shown++
	}
	return rows, shown, false
}
