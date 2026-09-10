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

// borderChar is the left spine of a tool block, one column wide.
const borderChar = "│"

// Box renders a full-width, left-bordered container for one tool's result or
// diff. Besides a single-column border each row is expected to already carry
// its own background and to span width-1 columns; the border column is drawn
// in borderColor over BlockBg, so a box reads as one pane with a coloured
// spine — green for a successful edit, red for a failed one, the tool's own
// colour while it is still running. The returned string ends with a newline.
func Box(rows []string, borderColor string) string {
	if len(rows) == 0 {
		return ""
	}
	border := lipgloss.NewStyle().
		Width(1).
		Background(lipgloss.Color(BlockBg)).
		Foreground(lipgloss.Color(borderColor)).
		Render(borderChar)
	var sb strings.Builder
	for _, row := range rows {
		sb.WriteString(border)
		sb.WriteString(row)
		sb.WriteString("\n")
	}
	return sb.String()
}
