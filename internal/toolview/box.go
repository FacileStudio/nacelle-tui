package toolview

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// BlockBg is the full-width backdrop a tool's result or diff block is drawn
// on. It is the same colour every line of the box — the border column and the
// content both — so the whole block reads as one continuous pane that stands
// out from the surrounding transcript.
const BlockBg = "237"

// MatchBackground applies the block backdrop to a style, or leaves it bare when
// transparent blocks are on. Every pane background is chosen here, so the
// setting takes effect in the border and the content alike. The preference is
// threaded through the callers rather than read from module state so these
// renderers stay pure.
func MatchBackground(style lipgloss.Style, transparent bool) lipgloss.Style {
	if transparent {
		return style
	}
	return style.Background(lipgloss.Color(BlockBg))
}

// borderChar is the left spine of a tool block, one column wide. It is the
// left half-block (▌): a solid filled bar a full cell tall, so it reads as a
// genuinely thick pane edge no matter how the terminal's box-drawing glyphs
// set their weights. The light (│) and heavy (┃) verticals both render at the
// same hairline in many monospace fonts.
const borderChar = "▌"

// Box renders a full-width, left-bordered container for one tool's result or
// diff. Besides a single-column border each row is expected to already carry
// its own background and to span width-1 columns; the border column is drawn
// in borderColor over BlockBg, so a box reads as one pane with a coloured
// spine — green for a successful edit, red for a failed one, the tool's own
// colour while it is still running. transparent drops that backdrop, so the
// pane floats on the terminal's own background. width is the pane's content
// width: a row longer than it is wrapped here rather than left for the
// terminal to soft-wrap, because a terminal's own wrap continuation carries
// no border and the pane falls apart visually mid-line. Continuations keep
// the same border and background as the row they continue. Returns a string
// ending in a newline.
func Box(rows []string, borderColor string, transparent bool, width int) string {
	if len(rows) == 0 {
		return ""
	}
	border := MatchBackground(lipgloss.NewStyle().Width(1).Foreground(lipgloss.Color(borderColor)), transparent).Render(borderChar)
	var sb strings.Builder
	for _, row := range rows {
		for _, piece := range wrapRow(row, max(width-1, 1)) {
			sb.WriteString(border)
			sb.WriteString(piece)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// wrapRow breaks one painted row at limit cells, ANSI escapes preserved, so
// the pane's border survives a long line. A row already at or under the limit
// is passed through untouched.
func wrapRow(row string, limit int) []string {
	if lipgloss.Width(row) <= limit {
		return []string{row}
	}
	pieces := strings.Split(strings.TrimSuffix(ansi.Wrap(row, limit, ""), "\n"), "\n")
	if len(pieces) == 0 {
		return []string{row}
	}
	return pieces
}

// WrapRow breaks one painted row at limit cells for a caller that lays out
// its own borders — the alternate-screen hold on a resize. Continuations of
// a row that opens with the box border glyph carry that glyph plus a space,
// wrapped at limit minus the two cells the prefix costs, so a wrapped pane
// keeps its spine and still fits the width. ANSI escapes are preserved
// throughout.
func WrapRow(row string, limit int) []string {
	pieces := wrapRow(row, limit)
	if len(pieces) < 2 || !strings.HasPrefix(row, borderChar) {
		return pieces
	}
	prefix := borderChar + " "
	inner := limit - lipgloss.Width(prefix)
	out := []string{pieces[0]}
	for _, piece := range pieces[1:] {
		for _, line := range wrapRow(piece, max(inner, 1)) {
			out = append(out, prefix+line)
		}
	}
	return out
}
