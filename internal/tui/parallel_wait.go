package tui

// relaxAfterDispatch ends the parent run once a model-callable parallel
// fan-out has registered its batch. Detach already made the tool non-blocking;
// this is the other half — the model must not keep working a turn whose
// results it cannot read, because a detached fan-out's outcomes stream to
// PostDetached and never back to its stream. Cancelling closes the stream,
// settle runs, and the prompt returns to ready while the spinner rows keep
// ticking under it. The live prompt covers any concurrent work the person
// wants from the main thread; the completion path re-engages the model to
// synthesize when the fan-out finishes. A /parallel launch from an idle prompt
// never reaches here (busy is false) and is untouched.
func (m *Model) relaxAfterDispatch() {
	if !m.run.busy {
		return
	}
	m.run.cancel()
}
