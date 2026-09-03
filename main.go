// Command nacelle is a terminal client for a nacelle agent.
//
// It exists to be the SDK's first consumer. A terminal exercises every event
// kind a backend can produce — text, reasoning, a tool starting, a tool
// finishing, why a turn ended, what it cost — which is more of the contract
// than a headless caller touches, and it does so while someone is watching.
//
// It is deliberately small. Sessions, profiles, themes and panes are what a
// product grows; none of them tests the API, and the point of this one is to
// find out where the API is annoying before three consumers depend on it.
package main

import (
	"fmt"
	"os"
	"strings"
)

// defaultSystem is who the model is and how it answers — the one layer
// -system replaces outright, because a different persona is exactly what
// that flag is for. Everything augmentSystem appends survives it: where the
// root is and what a leading slash does are not a matter of taste.
//
// It stays this short on purpose. Codex ships two prompts for one harness —
// 6.6KB for the models post-trained on it, 24KB for general GPT-5 — and the
// extra seventeen kilobytes is autonomy, planning and answer-formatting that
// the tuned model already knew. Claude is that tuned model for this shape of
// tool, so the ceiling here is closer to pi's ~600 characters than to
// anyone's twenty. What is here is only what a terminal changes about
// answering. Be brief, because the transcript competes with the live region
// for rows. Prefer a path over pasted source, because a dumped file scrolls
// the answer off the screen. And do not claim a thing works unchecked, which
// is the one failure a person cannot catch by reading the reply.
const defaultSystem = `You are a terminal coding assistant. Read before you write, keep edits small, and say what you changed.

Answer as briefly as the question allows, and lead with the result rather than the reasoning that reached it. Refer to code by path and line instead of pasting it back — the person asking has the file open, and a transcript full of quoted source scrolls the answer out of view. When inspecting code or gathering context, run independent read-only tool calls in parallel rather than one by one. Do not say something works until you have checked that it does; where you could not check, say so.`

// version is stamped by goreleaser at build time; `go install` builds leave
// the fallback in place.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "nacelle:", unprefixed(err))
		os.Exit(1)
	}
}

// unprefixed drops the library's own name from a message this program is about
// to put its name in front of.
//
// nacelle's errors say which package refused, which is correct for a library:
// a caller with several dependencies needs to know which one turned it down.
// This program is that library's front end and has already written the name, so
// keeping both prints "nacelle: nacelle: backend ...". A backend's prefix is
// left alone on purpose, because "nacelle/openrouter" says which backend and
// that half is worth reading twice.
func unprefixed(err error) string {
	return strings.TrimPrefix(err.Error(), "nacelle: ")
}

// hasPrintFlag reports whether -print was set, for routing to headless mode
// before the full flag setup in run().
func hasPrintFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-print" || strings.HasPrefix(arg, "-print=") {
			return true
		}
	}
	return false
}

// stripPrintFlag removes -print and its argument from os.Args and returns
// the prompt value. It must run before flag.Parse.
func stripPrintFlag() string {
	value := ""
	filtered := make([]string, 0, len(os.Args))
	filtered = append(filtered, os.Args[0])
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-print" {
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
				value = os.Args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "-print=") {
			value = strings.TrimPrefix(arg, "-print=")
		} else {
			filtered = append(filtered, arg)
		}
	}
	os.Args = filtered
	return value
}
