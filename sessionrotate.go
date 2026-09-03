package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

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

func (l *sessionLog) rotate() {
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
