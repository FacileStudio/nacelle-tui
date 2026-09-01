package main

import (
	"fmt"
	"strings"
	"time"
)

// environment is what the model would otherwise have to guess about the
// machine it has been handed.
//
// The banner already tells the person running this where the root is and
// whether the approval gate is on; nothing told the model any of it. Every
// line here is a fact no tool schema carries and no amount of reasoning
// recovers — plus one rule about how to work, batching independent tool
// calls, which no single tool description can carry because it spans all
// of them. It lives here rather than in the default prompt so a custom
// -system persona keeps it too.
//
// The path rule earns its length. Two tools live in two different path
// universes: clean() (tools/file.go) strips a leading "/" rather than
// refusing it, so read_file "/etc/hosts" quietly becomes "etc/hosts" under
// the root and fails as missing, while run_command runs with cmd.Dir set to
// the same root but sees the whole filesystem, so the identical path works.
// A model that learns "absolute paths work here" from its first successful
// run_command is then wrong about every file tool, and the error it gets
// back says nothing about why.
//
// The second half of that is named only when -bash mounted the tool it is
// about. run_command is off by default, and a paragraph explaining a tool
// the model does not have teaches it to trust this section less.
//
// It is appended whatever -system says. A different persona does not move
// the root or change what a leading slash does.
//
// Whether anyone reviews a tool call is said either way, and the negative is
// the one that earns its line: with the gate off — the default — the model is
// the last check before an irreversible command, and that is a different job
// from proposing one to a person who will read it. The shell paragraph is
// what that changes in practice, and it is named only when -bash mounted the
// shell, for the same reason the path rule above is.
//
// now is a parameter rather than a call so the text is testable. Nothing
// else needs it — the system prompt is built once, at startup.
//
// The *bool fields are read without a nil check, the same way banner() reads
// them, because settings() cannot return one unset: defaults() fills every
// pointer and merge() only ever overwrites with a layer that mentioned the
// setting. A guard here would be dead code that outlives the reason anyone
// added it. A Config built by hand — a test's, most likely — has to fill them,
// which is what withApproval exists for.
func environment(config Config, now time.Time) string {
	var body strings.Builder
	body.WriteString("\n\n## This session\n\n")

	fmt.Fprintf(&body, "Working directory: %s\n\n", absolute(config.Root))
	body.WriteString("The file and search tools take paths relative to it and cannot reach outside " +
		"it: a leading \"/\" is stripped rather than refused, so \"/etc/hosts\" means \"etc/hosts\" " +
		"inside the working directory.")
	if *config.Bash {
		body.WriteString(" run_command starts in that same directory but sees the whole filesystem, " +
			"so an absolute path works there and nowhere else.")
	}
	body.WriteString("\n\n")

	fmt.Fprintf(&body, "Today is %s.\n\n", now.Format(time.DateOnly))
	body.WriteString("What you write is rendered as markdown in a terminal.\n")

	body.WriteString("\nWhen several tool calls are independent — none needs another's result — make them together in " +
		"one turn instead of one after another; wait only where a later call needs an earlier one's output.\n")

	if *config.ApproveTools {
		body.WriteString("\nEvery tool call is shown to the person running this before it runs. A " +
			"refusal is their decision, not a failure to route around.\n")
	} else {
		body.WriteString("\nTool calls run the moment you make them. Nobody sees one first.\n")
	}

	if *config.Bash {
		body.WriteString("\nrun_command is a real shell, running with this process's own privileges " +
			"and nothing confining it to the working directory. Anything irreversible — " +
			"git reset --hard, git checkout --, a force push, rm on a path you did not create — " +
			"is worth a sentence to the person first rather than an apology after. Uncommitted " +
			"changes you did not make are theirs, not yours to tidy up.\n")

		body.WriteString("\nNEVER use sed, awk, perl -i or any other shell command to edit files. " +
			"Always use the edit_file or write_file tool for file edits — they produce a " +
			"visible diff I can review, and shell commands do not.\n")
	}
	return body.String()
}
