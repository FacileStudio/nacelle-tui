package diagnostics

import (
	"strings"
	"testing"
)

func TestParseKeepsOnlyErrorFindings(t *testing.T) {
	raw := strings.Join([]string{
		"a.go:1:1: info: style nit [gen.thing]",
		"b.go:2:3: warn: almost there [gen.thing]",
		"a.go:4:5: error: undefined: foo [go.typecheck]",
		"this line is not a finding",
		"c.go:6:7: error: cannot parse: c.go:8:12: expected ')', found 'EOF' [go.parse]",
		"",
	}, "\n")
	fs := parse(raw)
	if len(fs) != 2 {
		t.Fatalf("parse got %d findings, want 2: %v", len(fs), fs)
	}
	if fs[0].loc != "a.go:4:5" || fs[0].rule != "go.typecheck" || fs[0].msg != "undefined: foo" {
		t.Fatalf("first finding = %+v", fs[0])
	}
	if fs[1].msg != "cannot parse: c.go:8:12: expected ')', found 'EOF'" || fs[1].rule != "go.parse" {
		t.Fatalf("second finding = %+v", fs[1])
	}
	if want := "c.go:6:7: cannot parse: c.go:8:12: expected ')', found 'EOF'"; fs[1].line() != want {
		t.Fatalf("line = %q, want %q", fs[1].line(), want)
	}
}
