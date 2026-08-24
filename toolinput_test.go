package main

import (
	"encoding/json"
	"errors"
	"testing"
)

// The table covers the two jobs strictObject has: refusing ambiguity wherever
// it hides, and telling a caller apart from a call that simply has nothing to
// show. duplicate says which of the two outcomes is expected, because only one
// of them makes the approval gate fail closed.
type strictCase struct {
	name      string
	raw       string
	fields    []string
	duplicate string
	wantErr   bool
}

// strictCases is table data rather than a literal inside the test, so the test
// itself stays the four lines of what it asserts. Every entry is a shape a
// model has produced or plausibly will.
var strictCases = []strictCase{
	{
		name:   "a clean object keeps every field",
		raw:    `{"command":"ls","cwd":"/tmp"}`,
		fields: []string{"command", "cwd"},
	},
	{
		name:      "the rm -rf case",
		raw:       `{"command":"ls","command":"rm -rf /"}`,
		duplicate: "command",
		wantErr:   true,
	},
	{
		name:      "a duplicate nested inside an object",
		raw:       `{"tool":{"path":"a.txt","path":"b.txt"}}`,
		duplicate: "path",
		wantErr:   true,
	},
	{
		name:      "a duplicate inside an array element",
		raw:       `{"edits":[{"path":"a.txt"},{"path":"a.txt","path":"/etc/passwd"}]}`,
		duplicate: "path",
		wantErr:   true,
	},
	{
		name:   "the same key in sibling objects is ordinary JSON",
		raw:    `{"edits":[{"path":"a.txt"},{"path":"b.txt"}]}`,
		fields: []string{"edits"},
	},
	{
		name:    "a non-object top level",
		raw:     `["ls","rm -rf /"]`,
		wantErr: true,
	},
	{
		name:    "null",
		raw:     `null`,
		wantErr: true,
	},
	{
		name:    "empty input",
		raw:     ``,
		wantErr: true,
	},
	{
		name:    "malformed JSON",
		raw:     `{"command":`,
		wantErr: true,
	},
	{
		name:   "an object with no arguments at all",
		raw:    `{}`,
		fields: []string{},
	},
}

func TestStrictObject(t *testing.T) {
	for _, tc := range strictCases {
		t.Run(tc.name, func(t *testing.T) {
			fields, err := strictObject([]byte(tc.raw))
			checkRefusal(t, tc, fields, err)
			checkFields(t, tc, fields)
		})
	}
}

// checkRefusal is the half of the table about what strictObject would not
// stand behind: that it refused when it had to, that it handed back nothing
// renderable when it did, and that a duplicate is distinguishable from a call
// with no arguments — which is the distinction the approval gate acts on.
func checkRefusal(t *testing.T, tc strictCase, fields map[string]json.RawMessage, err error) {
	t.Helper()

	if tc.wantErr && err == nil {
		t.Fatalf("strictObject(%s) accepted input it must refuse", tc.raw)
	}
	if !tc.wantErr && err != nil {
		t.Fatalf("strictObject(%s) refused valid input: %v", tc.raw, err)
	}
	if err != nil && fields != nil {
		t.Errorf("a refused input still handed back %d fields to render", len(fields))
	}

	isDuplicate := errors.Is(err, errDuplicateKey)
	if (tc.duplicate != "") != isDuplicate {
		t.Fatalf("errors.Is(err, errDuplicateKey) = %v, want %v — got %v",
			isDuplicate, tc.duplicate != "", err)
	}
	if tc.duplicate == "" {
		return
	}
	if want := `duplicate key "` + tc.duplicate + `" in tool input`; err.Error() != want {
		t.Errorf("error = %q, want %q — the key has to be named or the refusal is unreadable", err, want)
	}
}

// checkFields is the other half: an accepted input has to arrive as the fields
// a caller will name the call by, or the refusal is the only thing this does.
func checkFields(t *testing.T, tc strictCase, fields map[string]json.RawMessage) {
	t.Helper()

	if tc.fields == nil {
		return
	}
	if len(fields) != len(tc.fields) {
		t.Fatalf("got %d fields, want %d", len(fields), len(tc.fields))
	}
	for _, key := range tc.fields {
		if _, ok := fields[key]; !ok {
			t.Errorf("field %q went missing", key)
		}
	}
}
