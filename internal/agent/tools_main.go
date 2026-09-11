package agent

import (
	"fmt"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/tools"

	"github.com/FacileStudio/nacelle-tui/internal/tasks"
)

// localTools opens the file and command tools and, when asked, adds the web
// one — the caller owns closing the returned Set.
//
// Every failure after tools.New succeeds closes the Set on the way out.
// Without that it is dropped on the floor: the caller's own defer only ever
// sees the nil this returns on an error, so the *os.Root behind it — a real
// descriptor — outlives the only reference to it. The process exits moments
// later today and nothing notices, which is the kind of leak that becomes
// real the first time something rebuilds a tool set without restarting.
//
// The handle is held in a plain local rather than in the returned one, and
// that is load-bearing rather than style. A named result is assigned by the
// return statement *before* any deferred function runs, so with the Set named
// there, `return nil, nil, err` had already overwritten it — the deferred
// close then panicked on a nil receiver instead of releasing anything. The
// error paths still hand the caller nil, which is what it wants; only the
// closing needs a name the returns cannot reach.
func localTools(config Config) (_ *tools.Set, local []nacelle.Tool, err error) {
	opened, err := tools.New(tools.Config{Root: config.Root, AllowBash: *config.Bash, StrictConfinement: *config.StrictConfinement})
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", config.Root, err)
	}

	local, err = opened.Tools()
	if err != nil {
		opened.Close()
		return nil, nil, fmt.Errorf("building the tool set: %w", err)
	}

	var reaching []nacelle.Tool
	if reaching, err = webTools(config); err != nil {
		opened.Close()
		return nil, nil, err
	}
	local = append(local, reaching...)

	return opened, local, nil
}

// withTasks mounts the plan tool when the settings ask for one. It is
// deliberately not part of the set localTools builds. The SDK gives a nested
// run the parent's tools minus only the delegate tool itself, so anything
// localTools returns is inherited by a delegate whether that makes sense or
// not. The plan is drawn above the prompt on the one screen there is; a
// delegate calling this would replace what the parent wrote, and nothing
// afterwards puts it back.
//
// So it goes on after withParallelAgents has taken its copy. Same set, one line
// later, and the delegate never sees it.
func withTasks(config Config, local []nacelle.Tool) []nacelle.Tool {
	if !*config.Tasks {
		return local
	}
	return append(local, tasks.NewTool())
}
