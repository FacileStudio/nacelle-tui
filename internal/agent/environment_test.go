package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

func defaults() Config {
	return settings.Defaults("")
}

func withApproval(on bool) Config {
	config := defaults()
	bash := true
	config.ApproveTools, config.Bash = &on, &bash
	return config
}

func TestEnvironmentNamesTheRootAbsolutely(t *testing.T) {
	config := withApproval(false)
	config.Root = "."

	got := environment(config, time.Now())

	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, want) {
		t.Errorf("environment() = %q, want the root resolved to %q rather than left as \".\"", got, want)
	}
}

func TestEnvironmentWarnsThatAbsolutePathsOnlyWorkInRunCommand(t *testing.T) {
	got := environment(withApproval(false), time.Now())

	for _, want := range []string{"absolute", "run_command"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to mention %q", got, want)
		}
	}
}

func TestEnvironmentDatesTheSession(t *testing.T) {
	got := environment(withApproval(false), time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC))

	if !strings.Contains(got, "2026-08-20") {
		t.Errorf("environment() = %q, want it to carry the date it was given", got)
	}
}

func TestEnvironmentSaysWhetherAnyoneReviewsAToolCall(t *testing.T) {
	on := environment(withApproval(true), time.Now())
	if !strings.Contains(on, "refusal") {
		t.Errorf("environment() = %q, want the approval gate explained when it is on", on)
	}
	if strings.Contains(on, "Nobody sees one first") {
		t.Errorf("environment() = %q, want no unreviewed warning while the gate is on", on)
	}

	if off := environment(withApproval(false), time.Now()); !strings.Contains(off, "Nobody sees one first") {
		t.Errorf("environment() = %q, want the model told nothing reviews a call with the gate off", off)
	}
}

func TestEnvironmentTeachesBatchingIndependentToolCalls(t *testing.T) {
	got := environment(withApproval(false), time.Now())

	for _, want := range []string{"independent", "one turn"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to teach batching with %q", got, want)
		}
	}
}

func TestEnvironmentSaysAParallelFanOutEndsTheTurn(t *testing.T) {
	got := environment(withApproval(false), time.Now())

	for _, want := range []string{"non-blocking and return-control", "ends your turn"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to tell the model a dispatched fan-out is the point to stop, with %q", got, want)
		}
	}
}

func TestEnvironmentWarnsAboutIrreversibleCommandsOnlyWithBash(t *testing.T) {
	if got := environment(withApproval(false), time.Now()); !strings.Contains(got, "git reset --hard") {
		t.Errorf("environment() = %q, want the irreversible commands named when the shell is on", got)
	}

	config := withApproval(false)
	off := false
	config.Bash = &off
	if got := environment(config, time.Now()); strings.Contains(got, "git reset --hard") {
		t.Errorf("environment() = %q, want no shell warning when there is no shell", got)
	}
}

func TestAugmentSystemAlwaysAppendsTheEnvironment(t *testing.T) {
	off := false
	config := defaults()
	config.System = "You are something else entirely."
	config.ProjectContext, config.Skills = &off, &off

	augmentSystem(&config)

	if !strings.Contains(config.System, "Working directory:") {
		t.Errorf("System = %q, want the session's own facts appended regardless of the switches", config.System)
	}
}

func TestEnvironmentMentionsRunCommandOnlyWhenBashIsMounted(t *testing.T) {
	config := withApproval(false)
	off := false
	config.Bash = &off

	if got := environment(config, time.Now()); strings.Contains(got, "run_command") {
		t.Errorf("environment() = %q, want no word about run_command when -bash is off", got)
	}
}

func TestEnvironmentDescribesConfinementBasedOnStrictConfinement(t *testing.T) {

	on := environment(withApproval(false), time.Now())
	if !strings.Contains(on, "absolute paths") {
		t.Errorf("environment() without confinement = %q, want mention of absolute-path handling", on)
	}
	if !strings.Contains(on, "no confinement") {
		t.Errorf("environment() without confinement = %q, want \"no confinement\" mentioned", on)
	}

	with := withApproval(false)
	confined := true
	with.StrictConfinement = &confined
	withConfined := environment(with, time.Now())
	if !strings.Contains(withConfined, "cannot reach outside") {
		t.Errorf("environment() with confinement = %q, want confinement warning", withConfined)
	}
	if strings.Contains(withConfined, "no confinement") {
		t.Errorf("environment() with confinement = %q, should not say \"no confinement\"", withConfined)
	}
}
