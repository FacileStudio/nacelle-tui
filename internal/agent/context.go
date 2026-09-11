package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// htmlComment is the authoring-note syntax markdown files carry. It is
// stripped before a file reaches the prompt: research on layered
// instructions validated the free win, and a note addressed to a future
// editor of the file is not an instruction to the model.
var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// instrumentFile is one CLAUDE.md or AGENTS.md found on the walk, kept
// alongside its path so the rendered output can say where it came from.
//
// global marks the one entry that did not come from the walk — ~/.agents/
// AGENTS.md — so renderLevels can label it as what it is rather than as a
// project's own instructions.
type instrumentFile struct {
	path, content string
	global        bool
}

// projectContext finds every CLAUDE.md and AGENTS.md between root and the
// filesystem root, plus ~/.agents/AGENTS.md if it exists, and returns them
// concatenated as extra system-prompt text — most general first, most
// specific last, empty string if none exist — alongside how many files
// that was, for the banner to summarize rather than repeat.
//
// It walks up from root rather than down from it, because the file that
// matters is the one describing the project the client was launched inside,
// which is usually an ancestor of the working directory a person actually
// typed, not something buried under it. Every match at every level is kept,
// not just the nearest: a monorepo's root CLAUDE.md carries suite-wide
// convention and a subdirectory's carries what is specific to it, and a
// reader who wrote both meant both read together.
//
// ~/.agents/AGENTS.md is the AGENTS.md standard's own global-base location —
// the file Codex, Cursor, Copilot, Gemini and pi (at its own equivalent,
// ~/.pi/agent/AGENTS.md) already read, stewarded cross-vendor rather than by
// any one tool. That is exactly why it is read here and a user's own
// ~/.claude/CLAUDE.md is not: CLAUDE.md is written for Claude Code
// specifically and references its tools, hooks and slash commands, where an
// AGENTS.md at the standard's own path is written, by the convention's own
// premise, to make sense to whichever agent finds it — the two are not the
// same kind of risk.
//
// This is deliberately not trust-gated the way project-local settings,
// skills or extensions would be if this package ever grows them. A context
// file is one more piece of text the model reads, no different in kind from
// a file it opens with a tool or a command's output — gating that one path
// while every other file the model reads stays ungated would not close a
// real hole, only add friction. See pi's own security docs: "context files
// ... are loaded regardless of project trust unless context loading is
// disabled" — the trust boundary there is reserved for things that change
// what the agent itself can do, which nothing this package reads today does.
func projectContext(root string) (string, int) {
	levels, seen := instrumentLevels(root)
	if global := globalInstructions(seen); len(global) > 0 {
		levels = append(levels, global)
	}
	count := 0
	for _, level := range levels {
		count += len(level)
	}
	return renderLevels(levels), count
}

// realpath resolves symlinks so the same file reached through two paths —
// most often a CLAUDE.md symlinked to AGENTS.md, the Mycelium setup — is
// recognised as one file and read once. A path that cannot be resolved falls
// back to itself: the read will simply fail later, as it already did.
func realpath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// instrumentLevels walks from root to the filesystem root, returning every
// directory that held a CLAUDE.md or an AGENTS.md, closest to root first and
// the filesystem root last — the reverse of the order the caller wants, so
// renderLevels is what turns it around.
//
// One file reaching the model twice is a prompt bloated by duplicates, so a
// resolved path is admitted once across the whole walk: the same inode behind
// two names, at the same level or two levels up, is read from the first
// encounter only.
func instrumentLevels(root string) ([][]instrumentFile, map[string]bool) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil
	}

	seen := map[string]bool{}
	var levels [][]instrumentFile
	for dir := abs; ; {
		if here := instrumentsIn(dir, seen); len(here) > 0 {
			levels = append(levels, here)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return levels, seen
		}
		dir = parent
	}
}

// globalInstructions reads ~/.agents/AGENTS.md, the one file this package
// reads outside the walk from root. No home directory or no file there is
// not an error — most machines will have neither, or won't have adopted the
// convention yet, and that is the ordinary case, not a degraded one.
//
// The walk already read the same real file — a project AGENTS.md symlinked
// to the global one, say — the global copy is skipped: the walk's level keeps
// its own label, and the content is not in the prompt twice.
func globalInstructions(seen map[string]bool) []instrumentFile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".agents", "AGENTS.md")
	if seen[realpath(path)] {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return []instrumentFile{{path: path, content: string(raw), global: true}}
}

// instrumentsIn reads CLAUDE.md and AGENTS.md from one directory, in that
// order. Either can be absent, and a file that exists but cannot be read is
// treated the same as one that was never there — one unreadable file is not
// a reason to fail the whole walk.
//
// Files already admitted earlier in the walk — the same real file under
// another name or another level — are skipped, not read twice.
func instrumentsIn(dir string, seen map[string]bool) []instrumentFile {
	var here []instrumentFile
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(dir, name)
		key := realpath(path)
		if seen[key] {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw = htmlComment.ReplaceAll(raw, nil)
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		seen[key] = true
		here = append(here, instrumentFile{path: path, content: string(raw)})
	}
	return here
}

// renderLevels concatenates levels found closest-to-root-first into
// root-to-leaf text, reversing instrumentLevels' walk order so the level
// nearest where projectContext was called — the most specific one — ends up
// last, immediately in front of whatever the model reads next. The global
// level, appended after the walk by projectContext, lands first: more
// general than even the filesystem root.
func renderLevels(levels [][]instrumentFile) string {
	var body strings.Builder
	for _, level := range slices.Backward(levels) {
		for _, f := range level {
			label := "Project instructions"
			if f.global {
				label = "Global instructions"
			}
			fmt.Fprintf(&body, "\n\n## %s from %s\n\n%s", label, f.path, f.content)
		}
	}
	return body.String()
}
