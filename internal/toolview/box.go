package toolview

import (
	"strings"

	"charm.land/lipgloss/v2"
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
// pane floats on the terminal's own background. Returns a string ending in a
// newline.
func Box(rows []string, borderColor string, transparent bool) string {
	if len(rows) == 0 {
		return ""
	}
	border := MatchBackground(lipgloss.NewStyle().Width(1).Foreground(lipgloss.Color(borderColor)), transparent).Render(borderChar)
	var sb strings.Builder
	for _, row := range rows {
		sb.WriteString(border)
		sb.WriteString(row)
		sb.WriteString("\n")
	}
	return sb.String()
}
