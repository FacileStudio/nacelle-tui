package menu

import "testing"

func TestCommandWord(t *testing.T) {
	if got := CommandWord("/clear now"); got != "/clear" {
		t.Errorf("CommandWord = %q, want /clear", got)
	}
	if got := CommandWord("not a command"); got != "" {
		t.Errorf("CommandWord = %q, want empty", got)
	}
}

func TestAnyCommand(t *testing.T) {
	if got := AnyCommand("run /test now"); got != "/test" {
		t.Errorf("AnyCommand = %q, want /test", got)
	}
	if got := AnyCommand("no command"); got != "" {
		t.Errorf("AnyCommand = %q, want empty", got)
	}
}

func TestInsertPick(t *testing.T) {
	if got := InsertPick("/cl", "/clear"); got != "/clear " {
		t.Errorf("InsertPick = %q, want /clear ", got)
	}
}

func TestReplaceCommandPreservesSurroundingText(t *testing.T) {
	want := "please run /clear  now"
	if got := ReplaceCommand("please run /cl now", "/clear"); got != want {
		t.Errorf("ReplaceCommand = %q, want %q", got, want)
	}
}

func TestReplaceCommandFromStartOfLine(t *testing.T) {
	want := "/clear  now"
	if got := ReplaceCommand("/cl now", "/clear"); got != want {
		t.Errorf("ReplaceCommand = %q, want %q", got, want)
	}
}

func TestReplaceCommandCommandAtEndOfValue(t *testing.T) {
	want := "please run /clear "
	if got := ReplaceCommand("please run /cl", "/clear"); got != want {
		t.Errorf("ReplaceCommand = %q, want %q", got, want)
	}
}

func TestReplaceCommandWithNoSlash(t *testing.T) {
	want := "hello world"
	if got := ReplaceCommand(want, "/clear"); got != want {
		t.Errorf("ReplaceCommand(%q) = %q, want it unchanged", want, got)
	}
}
