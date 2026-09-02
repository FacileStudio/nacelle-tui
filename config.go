// config.go — type aliases and shim functions.
//
// Config and the types below it alias the settings package, keeping bare names
// for every file that already uses them. The shim functions bridge main's
// naming convention to the settings package API.

package main

import s "github.com/FacileStudio/nacelle-tui/internal/settings"

type (
	Config    = s.Config
	Toggles   = s.Toggles
	UI        = s.UI
	Reasoning = s.Reasoning
	Web       = s.Web
	Discovery = s.Discovery
	Sources   = s.Sources
	HookSpec  = s.HookSpec
)

type hookConfig []HookSpec

const ConfigFile = s.ConfigFile

func settings(flags Config) (Config, error) {
	return s.Settings(defaultSystem, flags)
}

func defaults() Config { return s.Defaults("") }

var derefBool = s.DerefBool
