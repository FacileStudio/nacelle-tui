package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// errDuplicateKey marks the one refusal a caller has to act on, as opposed to
// the ones it merely has to render around.
//
// A tool call with no arguments is ordinary — it arrives as empty bytes, as
// null, or as {} — so a gate that treated every strictObject error as danger
// would refuse every no-argument tool in the session. Only ambiguity is
// dangerous, and errors.Is against this is how a caller tells the two apart.
var errDuplicateKey = errors.New("duplicate key")

// strictObject decodes model-produced tool input into its top-level fields,
// refusing anything whose meaning depends on which duplicate key you happen to
// read.
//
// encoding/json accepts a repeated object key and silently keeps the last one.
// That is a display/decision split waiting to happen: given
// {"command":"ls","command":"rm -rf /"} one reader can show ls while another
// runs rm -rf /, and this client's whole job at an approval prompt is that the
// thing on screen is the thing being authorised. The bytes come from a model,
// so this is a trust boundary and the answer is refusal, not a preference for
// the first key or the last.
//
// Go 1.27's encoding/json/v2 does reject duplicate names outright, and it is
// the obvious answer — but go.mod declares go 1.26.4, where that package only
// exists under GOEXPERIMENT=jsonv2. Importing it would make this file build on
// one toolchain and not the other, so the walk below stands in for it. It is
// forty lines and no dependency, which is the price of the go directive
// staying where it is.
func strictObject(raw []byte) (map[string]json.RawMessage, error) {
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

// scanValue consumes exactly one JSON value, having already been handed that
// value's opening token, and reports the first repeated key it finds anywhere
// beneath it.
//
// It recurses through arrays as well as objects because the ambiguity is not a
// property of the top level. {"edits":[{"path":"a.txt","path":"b.txt"}]} reads
// as innocent from the outside and is the same lie one layer down, so the walk
// has to reach every object in the value or it is a check that only catches
// the careless version of the attack.
//
// Each object gets its own seen set rather than one shared across the whole
// walk, because the same key appearing in two sibling objects is normal JSON
// and refusing it would refuse most real tool calls.
func scanValue(decoder *json.Decoder, opening json.Token) error {
	switch opening {
	case json.Delim('{'):
		return scanObject(decoder)
	case json.Delim('['):
		return scanArray(decoder)
	}
	return nil
}

// scanObject walks the members of one object, whose opening brace has already
// been read, and refuses the first name it sees twice.
//
// The seen set is this object's alone. The same key appearing in two sibling
// objects is ordinary JSON — every element of an edits array carries a path —
// and a set shared across the walk would refuse most real tool calls.
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

// claim records one name against the object it belongs to, and is where the
// refusal itself lives: a name already in the set is the ambiguity this whole
// file exists to catch.
//
// The non-string branch cannot happen in valid JSON, where a member name is
// always a string. It is here because the token is an any and the type
// assertion needs an else, and an error naming what arrived beats a panic on
// input a model produced.
func claim(seen map[string]bool, key json.Token) error {
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("tool input has a non-string object key %v", key)
	}
	if seen[name] {
		return fmt.Errorf("%w %q in tool input", errDuplicateKey, name)
	}
	seen[name] = true
	return nil
}

// scanMember walks the value a name was bound to, which is the step that makes
// this a walk rather than a check of the top level. A repeated key one layer
// down is the same lie as a repeated key at the top, and reads as innocent
// from outside.
func scanMember(decoder *json.Decoder) error {
	value, err := decoder.Token()
	if err != nil {
		return err
	}
	return scanValue(decoder, value)
}

// scanArray walks the elements of one array, whose opening bracket has already
// been read. It exists because an array is where an object hides.
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
