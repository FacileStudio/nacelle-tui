package agent

import "github.com/FacileStudio/nacelle"

func resolveCompactAt(compactAt int64, backend nacelle.Backend) int64 {
	if compactAt == 100000 && backend.Capabilities().ContextWindow > 0 {
		return backend.Capabilities().ContextWindow * 3 / 4
	}
	return compactAt
}
