package diagnostics

import (
	"context"

	"github.com/FacileStudio/nacelle"
)

type toolInput struct {
	Path string `json:"path" jsonschema:"description=File to check, relative to the working directory"`
	Repo bool   `json:"repo,omitempty" jsonschema:"description=Check the whole session root instead of the one file"`
}

// Tool builds the model-facing diagnostics tool. The model checks one file by
// passing path, or sweeps the session root with repo; what it gets back is
// Run's rendering, and the input schema is generated from toolInput exactly
// the way the SDK's own tools generate theirs. The tool is read only: filet
// check never mutates the workspace.
func Tool() nacelle.Tool {
	built, _ := nacelle.NewToolWithOptions("diagnostics",
		"Run the filet quality checker and return compiler-style findings as file:line:col lines, errors only; an empty result means the code is clean. Pass path to check one file, or repo to check the whole session root and ignore path. Use it to confirm an edit fixed what it broke or to sweep the tree before finishing.",
		runTool, nacelle.ToolOptions{ReadOnly: true})
	return built
}

func runTool(ctx context.Context, in toolInput) (string, error) {
	return Run(ctx, in.Path, in.Repo)
}
