package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FacileStudio/nacelle"
	s "github.com/FacileStudio/nacelle-tui/internal/settings"
)

// HooksFile is the project-level hooks file, read in addition to the
// `hooks:` entries in the user's own config.
const HooksFile = ".nacelle/hooks.yml"

// HookTrustFile records, per absolute path, the hash of the last hooks file
// trusted from there.
const HookTrustFile = "hooks.json"

// HookPointOf returns the nacelle.HookPoint for a YAML event string.
func HookPointOf(event string) nacelle.HookPoint {
	return s.HookPointOf(event)
}

// buildHooks turns config entries into live library hooks, refusing any
// spec that would otherwise fail silently mid-session.
func buildHooks(specs []HookSpec) (map[nacelle.HookPoint][]nacelle.Hook, error) {
	var hooks map[nacelle.HookPoint][]nacelle.Hook
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, err
		}
		var hook nacelle.Hook
		if spec.Async {
			hook = nacelle.Async(execHook(spec))
		} else {
			hook = nacelle.WithTimeout(spec.Duration(), execHook(spec))
		}
		hooks = appendHook(hooks, HookPointOf(spec.On), hook)
	}
	return hooks, nil
}

func appendHook(hooks map[nacelle.HookPoint][]nacelle.Hook, p nacelle.HookPoint, hook nacelle.Hook) map[nacelle.HookPoint][]nacelle.Hook {
	if hooks == nil {
		hooks = map[nacelle.HookPoint][]nacelle.Hook{}
	}
	hooks[p] = append(hooks[p], hook)
	return hooks
}

// hookPayload is the process contract's input: one JSON object on stdin.
type hookPayload struct {
	Event  string `json:"event"`
	Tool   string `json:"tool"`
	Input  string `json:"input"`
	Result string `json:"result,omitempty"`
	Retry  bool   `json:"retry"`
}

// sessionHooks resolves every hooks layer in one place.
func sessionHooks(config Config) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
	hooks, err := buildHooks(config.Hooks)
	if err != nil {
		return nil, "", err
	}

	project, notice, err := loadProjectHooks(config.Root, *config.TrustHooks)
	if err != nil {
		return nil, "", err
	}
	for p, list := range project {
		for _, hook := range list {
			hooks = appendHook(hooks, p, hook)
		}
	}
	return hooks, notice, nil
}

// loadProjectHooks reads <root>/.nacelle/hooks.yml through the trust gate.
func loadProjectHooks(root string, trustNew bool) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
	path := filepath.Join(root, HooksFile)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", path, err)
	}

	specs, err := parseHooks(raw)
	if err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", path, err)
	}
	hooks, err := buildHooks(specs)
	if err != nil {
		return nil, "", fmt.Errorf("in %s: %w", path, err)
	}

	hash := contentHash(raw)
	trusted, err := hookIsTrusted(path, hash, trustNew)
	if err != nil {
		return nil, "", err
	}
	if !trusted {
		return nil, fmt.Sprintf(
			"This project defines hooks in %s (%d lines of commands that run on every tool call) and they are not trusted yet.\n"+
				"Read them, then restart with -trust-hooks to approve this version.", HooksFile, bytes.Count(raw, []byte("\n"))), nil
	}
	return hooks, "", nil
}

// hookIsTrusted reports whether this exact file content has been approved.
func hookIsTrusted(path, hash string, trustNew bool) (bool, error) {
	store, err := loadHookTrust()
	if err != nil {
		return false, err
	}
	if record, seen := store[path]; seen && record.Hash == hash {
		return true, nil
	}
	if !trustNew {
		return false, nil
	}
	return true, saveHookTrust(store, path, hash)
}
