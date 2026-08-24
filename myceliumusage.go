package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// canonicalEvent is mycelium's session transport shape, as its internal/sessions
// package reads it. The two constant fields are not decoration: mycelium counts
// only an assistant message carrying usage, and drops everything else.
type canonicalEvent struct {
	Type      string         `json:"type"`
	Role      string         `json:"role"`
	Timestamp string         `json:"timestamp"`
	Agent     string         `json:"agent"`
	Machine   string         `json:"machine"`
	Project   string         `json:"project"`
	Branch    string         `json:"branch,omitempty"`
	Model     string         `json:"model,omitempty"`
	Usage     canonicalUsage `json:"usage"`
}

type canonicalUsage struct {
	Input      int64          `json:"input"`
	Output     int64          `json:"output"`
	CacheRead  int64          `json:"cacheRead"`
	CacheWrite int64          `json:"cacheWrite"`
	Cost       *canonicalCost `json:"cost,omitempty"`
}

// canonicalCost carries only the total. mycelium's shape splits cost per token
// class, and a backend that reports a single number cannot fill those in
// without inventing the split.
type canonicalCost struct {
	Total float64 `json:"total"`
}

// usageSink appends one canonical event per finished turn to mycelium's event
// feed. mycelium's `sessions scan` globs events/*/*.jsonl under its data dir, so
// writing files into events/nacelle/ is the entire integration — no API, no
// registration, no daemon to talk to. The session then appears in the
// dashboard's sessions tab as machine/nacelle, live while it is running.
//
// A nil sink records nothing, which is what a machine without mycelium gets:
// creating the tree there would leave a directory nothing ever reads.
type usageSink struct {
	dir     string
	machine string
	project string
	branch  string
	model   string
}

// newUsageSink returns nil unless mycelium's data directory already exists,
// which is the cheapest honest signal that mycelium is installed here. DATA_DIR
// is read for the same reason mycelium reads it: a test or a second instance
// points both halves at a temporary tree.
func newUsageSink(root, model string) *usageSink {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dataDir = filepath.Join(home, ".mycelium")
	}
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return nil
	}
	machine, _ := os.Hostname()
	project, branch := repoIdentity(root)
	return &usageSink{
		dir:     filepath.Join(dataDir, "events", "nacelle"),
		machine: machine,
		project: project,
		branch:  branch,
		model:   model,
	}
}

// record appends one turn's usage. KindTurn only: KindDone repeats the run
// total, and counting both would double every figure mycelium reports.
//
// Every failure is swallowed. Usage accounting is bookkeeping beside the
// session, and a full disk or a read-only home must not take down the thing
// the person is actually doing; the next turn writes again.
//
// Swallowed at one place, though, not wherever it happens. A write and a close
// are two ways to lose the same line, so appendLine joins them and this reads
// "if it failed, never mind" once — rather than ignoring two errors in two
// spellings and leaving a reader to work out whether either was deliberate.
func (s *usageSink) record(usage nacelle.Usage, now time.Time) {
	if s == nil {
		return
	}
	line, err := json.Marshal(s.canonical(usage, now))
	if err != nil {
		return
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(s.dir, now.UTC().Format("2006-01")+".jsonl")
	if err := appendLine(path, line); err != nil {
		return
	}
}

// appendLine adds one line to a file, reporting whether either half of that
// failed.
//
// The close is joined rather than deferred because a deferred close throws its
// error away, and on an append that error is the one that matters: a buffered
// write can succeed against a full disk and only fail as the descriptor is
// closed. Joining says the line was not written either way, which is the only
// thing the caller could act on.
func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, written := f.Write(append(line, '\n'))
	return errors.Join(written, f.Close())
}

// canonical is one turn's usage in the shape mycelium reads. It is split from
// record so that the writing is a page about writing: eighteen lines of struct
// literal in front of the file handling buried the part that can actually go
// wrong.
func (s *usageSink) canonical(usage nacelle.Usage, now time.Time) canonicalEvent {
	event := canonicalEvent{
		Type:      "message",
		Role:      "assistant",
		Timestamp: now.UTC().Format(time.RFC3339Nano),
		Agent:     "nacelle",
		Machine:   s.machine,
		Project:   s.project,
		Branch:    s.branch,
		Model:     s.model,
		Usage: canonicalUsage{
			Input:      usage.InputTokens,
			Output:     usage.OutputTokens,
			CacheRead:  usage.CacheReadTokens,
			CacheWrite: usage.CacheCreationTokens,
		},
	}
	if usage.Cost > 0 {
		event.Usage.Cost = &canonicalCost{Total: usage.Cost}
	}
	return event
}

// repoIdentity names the project and branch the way mycelium's own resolver
// does — origin's repository name first, the toplevel directory second. Two
// harnesses run in the same checkout have to produce the same project name or
// the dashboard shows one repository twice, so this mirrors resolveProject in
// mycelium's internal/sessions rather than picking something simpler.
func repoIdentity(root string) (string, string) {
	if root == "" {
		root = "."
	}
	project := filepath.Base(root)
	if abs, err := filepath.Abs(root); err == nil {
		project = filepath.Base(abs)
	}
	if out, err := gitIn(root, "rev-parse", "--show-toplevel"); err == nil && out != "" {
		project = filepath.Base(out)
	}
	if out, err := gitIn(root, "remote", "get-url", "origin"); err == nil {
		if name := repoNameFromRemote(out); name != "" {
			project = name
		}
	}
	branch, err := gitIn(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || branch == "HEAD" {
		branch = ""
	}
	return project, branch
}

func gitIn(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// repoNameFromRemote pulls the repository name out of a remote URL, in both
// the ssh and https spellings. It refuses anything that is not a name, so a
// malformed remote falls back to the directory rather than becoming a project
// called ".." or one with a space in it.
func repoNameFromRemote(url string) string {
	url = strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	if url == "" || url == "." || url == ".." || strings.ContainsAny(url, " \\") {
		return ""
	}
	return url
}
