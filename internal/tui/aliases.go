package tui

import (
	"github.com/FacileStudio/nacelle-tui/internal/diff"
	"github.com/FacileStudio/nacelle-tui/internal/sessions"
	"github.com/FacileStudio/nacelle-tui/internal/settings"
	"github.com/FacileStudio/nacelle-tui/internal/skills"
	"github.com/FacileStudio/nacelle-tui/internal/tasks"
	"github.com/FacileStudio/nacelle-tui/internal/tui/approval"
	"github.com/FacileStudio/nacelle-tui/internal/tui/layout"
	"github.com/FacileStudio/nacelle-tui/internal/tui/status"
	"github.com/FacileStudio/nacelle-tui/internal/tui/toolview"
)

type editChange = diff.EditChange
type skill = skills.Skill
type taskList = tasks.TaskList
type taskUpdate = tasks.TaskUpdate
type toolGroup = toolview.Group
type toolError = toolview.ToolError

type approvalDecision = approval.Decision
type approvalRequest = approval.Request

// Approvals aliases approval.Approvals.
type Approvals = approval.Approvals

// ApprovalRequest aliases approval.Request.
type ApprovalRequest = approval.Request

const (
	denied            = approval.Denied
	allowedOnce       = approval.AllowedOnce
	allowedForSession = approval.AllowedForSession
)

// Config aliases settings.Config.
type Config = settings.Config

var listSessionFiles = sessions.ListSessionFiles
var loadSession = sessions.LoadSession
var formatSessionEntry = sessions.FormatSessionEntry

var priorContents = diff.PriorContents
var renderDiff = diff.RenderDiff
var captureEdit = diff.CaptureEdit

var truncate = layout.Truncate
var unstyled = layout.Unstyled
var promptCap = layout.PromptCap

var shortTokens = status.ShortTokens
var waitingVerb = status.WaitingVerb
var lasted = status.Lasted
var waiting = status.WaitingPhrases()

const rephrase = status.Rephrase

const statusDone = "completed"
