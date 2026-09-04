package tui

import (
	"github.com/FacileStudio/nacelle-tui/internal/diff"
	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/settings"
	"github.com/FacileStudio/nacelle-tui/internal/skills"
	"github.com/FacileStudio/nacelle-tui/internal/tasks"
	"github.com/FacileStudio/nacelle-tui/internal/tui/toolview"
)

type editChange = diff.EditChange
type skill = skills.Skill
type taskList = tasks.TaskList
type taskUpdate = tasks.TaskUpdate
type toolGroup = toolview.Group
type toolError = toolview.ToolError

// Config aliases settings.Config.
type Config = settings.Config

var listSessionFiles = sessions.ListSessionFiles
var loadSession = sessions.LoadSession
var formatSessionEntry = sessions.FormatSessionEntry

var strictObject = toolview.StrictObject
var errDuplicateKey = toolview.ErrDuplicateKey

var priorContents = diff.PriorContents
var renderDiff = diff.RenderDiff
var captureEdit = diff.CaptureEdit

const statusDone = "completed"
