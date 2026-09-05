package diff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errDuplicateKey = errors.New("duplicate key")

func strictObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decodeObjectStart(decoder); err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		if err := decodeObjectField(decoder, fields); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func decodeObjectStart(decoder *json.Decoder) error {
	first, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return errors.New("tool input is empty")
	}
	if err != nil {
		return err
	}
	delim, ok := first.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("tool input is %T, want JSON object", first)
	}
	return nil
}

func decodeObjectField(decoder *json.Decoder, fields map[string]json.RawMessage) error {
	tok, err := decoder.Token()
	if err != nil {
		return err
	}
	key, ok := tok.(string)
	if !ok {
		return fmt.Errorf("object key is %T, want string", tok)
	}
	if _, seen := fields[key]; seen {
		return fmt.Errorf("duplicate key %q: %w", key, errDuplicateKey)
	}
	var val json.RawMessage
	if err := decoder.Decode(&val); err != nil {
		return err
	}
	fields[key] = val
	return nil
}
