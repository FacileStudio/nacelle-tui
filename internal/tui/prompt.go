package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const promptRows = 10

// minHeightRows is the shortest the input renders even when it holds a single
// line, so the field reads as a single line rather than a padded bar. The blank
// row above the whole field is added by the view assembly.
const minHeightRows = 1

// newPrompt builds the compose textarea. The prompt's gutter is a muted left
// spine (see promptBorder), so what you type opens one cell from that spine
// and a wrapped question reads as one bordered block rather than a floating
// field. placeholder is the ghost text shown while the prompt is empty. muted
// colours that spine and matches the palette's muted tone. Every row carries
// the same backdrop (see promptBackdrop), so a wrapped question holds its bar
// all the way down instead of dropping it on the rows below the cursor's.
func newPrompt(placeholder string, muted lipgloss.Style) textarea.Model {
	prompt := textarea.New()
	prompt.Placeholder = placeholder
	prompt.SetPromptFunc(1, promptBorder(muted))
	prompt.ShowLineNumbers = false
	prompt.DynamicHeight = true
	prompt.MinHeight = minHeightRows
	prompt.MaxHeight = promptRows
	prompt.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	prompt.SetVirtualCursor(false)
	backdrop := promptBackdrop()
	styles := prompt.Styles()
	styles.Focused.Base = styles.Focused.Base.Inherit(backdrop)
	styles.Blurred.Base = styles.Blurred.Base.Inherit(backdrop)
	prompt.SetStyles(styles)
	prompt.Focus()
	return prompt
}

// promptBackdrop is the flat ground the whole input field paints, so a wrapped
// question holds the same background on every row. The textarea only
// backdrops the cursor line and ends every row's own style in a full reset,
// so without it the rows of every earlier line fall back to the terminal
// default and a multi-line prompt reads as a patchwork. Base is inherited by
// every computed style, which is what puts the bar under all rows and states.
func promptBackdrop() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color("0"))
}

// promptBgClear resets the terminal background to default ahead of the border
// glyph, so the left spine never inherits the background of the row it sits
// on — notably the cursor line, which the textarea paints over its row. muted
// then picks the spine's foreground: the two escapes draw a muted ▌ on a
// transparent ground while the rest of the prompt keeps its own background.
const promptBgClear = "\x1b[49m"

// promptBorder is the gutter for every row: a single muted ▌ with no
// background, the same left-spine half-block the run_command boxes draw, so
// the input bar reads as a bordered pane. The single margin space the gutter
// used to be kept every row one cell from the left edge; the ▌ holds that
// one-column spacing and adds the border.
func promptBorder(muted lipgloss.Style) func(textarea.PromptInfo) string {
	border := promptBgClear + muted.Render("▌")
	return func(textarea.PromptInfo) string {
		return border
	}
}

func (m *Model) ask() tea.Cmd {
	question := strings.TrimSpace(m.prompt.Value())
	if question == "" {
		return nil
	}
	m.prompt.Reset()

	if !m.run.busy && !m.compacting {
		m.hist.Remember(question, m.Items())
		m.layout(m.windowHeight)
		return m.dispatch(question)
	}

	held := m.hist.Requeue(m.Items(), question)
	if !held {
		m.Add(question)
	}
	m.hist.Remember(question, m.Items())
	m.layout(m.windowHeight)
	return nil
}

func (m *Model) dispatch(line string) tea.Cmd {
	m.say(fromReader, line)

	started := tea.Cmd(nil)
	if cmd, ok := m.parseCommand(line); ok {
		started = cmd(m)
	} else {
		started = m.send(line)
	}
	return tea.Sequence(m.prints(), started)
}
