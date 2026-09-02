package main

import (
	"bytes"
	"errors"
	"io"

	"go.yaml.in/yaml/v4"
)

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
