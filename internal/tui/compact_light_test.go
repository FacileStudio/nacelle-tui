package tui

// Tests for the light lever (mask-before-summarize) and the thrash guard.

import (
	"strings"
	"testing"
)

func TestMaskLightLandsUnderAndReportsTheMask(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	ok := m.maskLight(evictCut)

	if !ok {
		t.Errorf("maskLight = false, want true once masking landed the conversation under the trigger threshold")
	}
	if m.size > m.compactAt+compactSlack {
		t.Errorf("size = %d after the mask, want under the trigger threshold %d", m.size, m.compactAt+compactSlack)
	}
	said := strings.Join(spoken(m), " ")
	if !strings.Contains(said, "✂ Compaction summary") || !strings.Contains(said, "masked") {
		t.Errorf("report = %q, want the mask described", said)
	}
	if strings.Contains(said, "summarized") {
		t.Errorf("report = %q, want no summary on a mask-only pass", said)
	}
}

func TestMaskLightEscalatesWhenItCannotLandUnder(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = 200_000
	evictCut := len(m.conversation) - keepCount(len(m.conversation))

	ok := m.maskLight(evictCut)

	if ok {
		t.Errorf("maskLight = true, want false when the evicted middle cannot yield enough to land under")
	}
	if m.size <= m.compactAt+compactSlack {
		t.Errorf("size = %d, want it left above the threshold so the caller escalates", m.size)
	}
	if said := strings.Join(spoken(m), " "); strings.Contains(said, "✂") {
		t.Errorf("said = %q, want no report when the mask did not complete the pass", said)
	}
}

// TestCompactBeforeSendRunsOnlyTheLightLever checks that a pre-flight that the
// mask alone clears does not escalate to a summary pass: it returns nil (no
// wait for an outcome) and lands the conversation under the threshold.
func TestCompactBeforeSendRunsOnlyTheLightLeverWhenItSuffices(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000

	cmd := m.compactBeforeSend(t.Context())

	if cmd != nil {
		t.Errorf("compactBeforeSend = a Cmd, want nil when masking alone cleared the threshold — no pass should be waiting")
	}
	if m.size > m.compactAt+compactSlack {
		t.Errorf("size = %d after the light lever, want under the trigger threshold", m.size)
	}
	if m.compacting {
		t.Errorf("compacting = true, want no pass started")
	}
}

func TestCheckThrashCountsNearMissesAndWarnsOnlyAtTheLimit(t *testing.T) {
	m := sized()
	m.size = m.compactAt + compactSlack + 1

	for i := range thrashLimit - 1 {
		m.checkThrash()
		if m.thrashed() {
			t.Errorf("thrashed after %d near-miss passes, want the guard to wait for %d", i+1, thrashLimit)
		}
		if said := strings.Join(spoken(m), " "); strings.Contains(said, "compaction keeps leaving") {
			t.Errorf("said = %q, want no stand-down warning before the limit", said)
		}
	}

	m.checkThrash()

	if !m.thrashed() {
		t.Errorf("thrashed = false after %d consecutive near-misses, want the guard stood down", thrashLimit)
	}
	if said := strings.Join(spoken(m), " "); !strings.Contains(said, "compaction keeps leaving") {
		t.Errorf("said = %q, want the stand-down warning at the limit", said)
	}
}

func TestCheckThrashResetsTheCounterWhenUnder(t *testing.T) {
	m := sized()
	m.thrashCount = thrashLimit - 1
	m.size = m.compactAt - 1

	m.checkThrash()

	if m.thrashCount != 0 {
		t.Errorf("thrashCount = %d after a pass landed under the threshold, want it reset", m.thrashCount)
	}
	if m.thrashed() {
		t.Errorf("thrashed = true after a pass landed under, want it cleared")
	}
}
