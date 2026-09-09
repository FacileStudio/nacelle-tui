package agent

import (
	"fmt"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/tools"
)

// webTools builds the one web tool that reaches past this machine: fetch,
// unless it has been turned off.
func webTools(config Config) ([]nacelle.Tool, error) {
	if !*config.Fetch {
		return nil, nil
	}

	reading, err := tools.WebFetch()
	if err != nil {
		return nil, fmt.Errorf("building the web fetch tool: %w", err)
	}
	return reading, nil
}
