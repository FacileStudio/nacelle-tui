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
	first, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil, errors.New("tool input is empty")
	}
	if err != nil {
		return nil, err
	}
	delim, ok := first.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("tool input is %T, want JSON object", first)
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("object key is %T, want string", tok)
		}
		if _, seen := fields[key]; seen {
			return nil, fmt.Errorf("duplicate key %q: %w", key, errDuplicateKey)
		}
		var val json.RawMessage
		if err := decoder.Decode(&val); err != nil {
			return nil, err
		}
		fields[key] = val
	}
	return fields, nil
}
