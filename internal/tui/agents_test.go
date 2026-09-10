package tui

import (
	"context"
	"iter"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// The warning belongs to the run that earned it. Left standing, it would
// accuse the next answer of being truncated too.
func TestANewQuestionClearsTheStopFromTheLastRun(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopMaxTokens})
	m.prompt.SetValue("again please")
	m.ask()
	defer m.run.cancel()

	if m.run.stop != "" {
		t.Errorf("stop = %q, want the previous run's reason cleared", m.run.stop)
	}
}

// silent is a backend that ends a run without saying anything, which is all a
// test that only cares about what asking does to the model needs.
type silent struct{}

func (silent) Name() string                       { return "silent" }
func (silent) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }

func (silent) CountTokens(context.Context, nacelle.Request) (int64, error) { return 0, nil }

func (silent) Stream(context.Context, nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

// answering is an agent that can be asked something. The model dereferences it
// as soon as a question is sent, so a test that calls ask needs a real one.
func answering(t *testing.T) *nacelle.Agent {
	t.Helper()

	agent, err := nacelle.New(nacelle.Config{Backend: silent{}, System: "be quiet"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return agent
}
