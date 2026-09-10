package sessions

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/FacileStudio/nacelle"
)

// RestoreAtLaunch returns the conversation to restore when the client starts:
// the session named by resume (an id or a file path) when one is given, else
// the newest session for root when auto is on. It returns the conversation,
// the display name of the session it came from, and a message to show when an
// explicit resume names nothing. A missing resume target surfaces as (nil, nil,
// message); a present-but-unloadable session returns as (nil, name, "").
func RestoreAtLaunch(resume, root string, auto bool) ([]nacelle.Message, string, string) {
	if resume != "" {
		path := ResolveSession(resume)
		if path == "" {
			return nil, "", "no session found for \"" + resume + "\""
		}
		return LoadSession(path), filepath.Base(path), ""
	}
	if auto {
		if root == "" {
			root = "."
		}
		files := ListSessionFiles(root)
		if len(files) > 0 {
			return LoadSession(files[0]), filepath.Base(files[0]), ""
		}
	}
	return nil, "", ""
}

func cleanup(f *os.File, path string) {
	if f != nil {
		if err := f.Close(); err != nil {
			return
		}
	}
	if err := os.Remove(path); err != nil {
		return
	}
}

func archiveSession(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	gzPath := path + ".gz"
	if _, err := os.Stat(gzPath); err == nil {
		return false
	}
	gzFile, err := os.OpenFile(gzPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	gz := gzip.NewWriter(gzFile)
	if _, err := gz.Write(content); err != nil {
		cleanup(gzFile, gzPath)
		return false
	}
	if err := gz.Close(); err != nil {
		cleanup(gzFile, gzPath)
		return false
	}
	if err := gzFile.Close(); err != nil {
		cleanup(nil, gzPath)
		return false
	}
	return os.Remove(path) == nil
}

func (l *SessionLog) rotate() {
	if !archiveSession(l.path) {
		l.lastSize = 0
		return
	}
	now := time.Now()
	name := now.UTC().Format("20060102T150405.000Z") + "-" + strconv.Itoa(os.Getpid()) + ".jsonl"
	l.path = filepath.Join(filepath.Dir(l.path), name)
	l.lastSize = 0
	l.write(sessionHeader{
		Version: 1,
		Started: now.Format(time.RFC3339Nano),
		Backend: l.backend,
		Model:   l.model,
		Root:    l.root,
	})
}
