package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/FacileStudio/nacelle"
	"go.yaml.in/yaml/v4"
)

// HooksFile is the project-level hooks file, read in addition to the
// `hooks:` entries in the user's own config.
const HooksFile = ".nacelle/hooks.yml"

// HookTrustFile records, per absolute path, the hash of the last hooks file
// trusted from there.
const HookTrustFile = "hooks.json"

// trustDir is where HookTrustFile lives — the first thing this package puts
// under ~/.nacelle/, which stays otherwise empty until something needs it.
func trustDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".nacelle"), nil
}

// parseHooks decodes one hooks file.
func parseHooks(raw []byte) ([]HookSpec, error) {
	var file struct {
		Hooks []HookSpec `yaml:"hooks"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return file.Hooks, nil
}

// BuildHooks turns config entries into live library hooks, refusing any
// spec that would otherwise fail silently mid-session.
func BuildHooks(specs []HookSpec) (map[nacelle.HookPoint][]nacelle.Hook, error) {
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

// SessionHooks resolves every hooks layer in one place.
func SessionHooks(config Config) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
	hooks, err := BuildHooks(config.Automation.Hooks)
	if err != nil {
		return nil, "", err
	}

	project, notice, err := LoadProjectHooks(config.Root, *config.TrustHooks)
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

// LoadProjectHooks reads <root>/.nacelle/hooks.yml through the trust gate.
func LoadProjectHooks(root string, trustNew bool) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
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
	hooks, err := BuildHooks(specs)
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
