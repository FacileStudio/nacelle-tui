package toolview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrDuplicateKey marks a tool input containing duplicate keys.
var ErrDuplicateKey = errors.New("duplicate key")

// StrictObject decodes tool input JSON, rejecting duplicate object keys.
func StrictObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))

	first, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil, errors.New("tool input is empty")
	}
	if err != nil {
		return nil, err
	}
	if first != json.Delim('{') {
		return nil, errors.New("tool input is not a JSON object")
	}
	if err := scanValue(decoder, first); err != nil {
		return nil, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func scanValue(decoder *json.Decoder, opening json.Token) error {
	switch opening {
	case json.Delim('{'):
		return scanObject(decoder)
	case json.Delim('['):
		return scanArray(decoder)
	}
	return nil
}

func scanObject(decoder *json.Decoder) error {
	seen := map[string]bool{}
	for {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		if key == json.Delim('}') {
			return nil
		}
		if err := claim(seen, key); err != nil {
			return err
		}
		if err := scanMember(decoder); err != nil {
			return err
		}
	}
}

func claim(seen map[string]bool, key json.Token) error {
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("tool input has a non-string object key %v", key)
	}
	if seen[name] {
		return fmt.Errorf("%w %q in tool input", ErrDuplicateKey, name)
	}
	seen[name] = true
	return nil
}

func scanMember(decoder *json.Decoder) error {
	value, err := decoder.Token()
	if err != nil {
		return err
	}
	return scanValue(decoder, value)
}

func scanArray(decoder *json.Decoder) error {
	for {
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if value == json.Delim(']') {
			return nil
		}
		if err := scanValue(decoder, value); err != nil {
			return err
		}
	}
}
