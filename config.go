// config.go — type aliases and shim functions.
//
// Config and the types below it alias the settings package, keeping bare names
// for every file that already uses them. The shim functions bridge main's
// naming convention to the settings package API.

package main

import s "github.com/FacileStudio/nacelle-tui/internal/settings"

// Config is an alias for settings.Config.
type Config = s.Config

// Toggles is an alias for settings.Toggles.
type Toggles = s.Toggles

// UI is an alias for settings.UI.
type UI = s.UI

// Reasoning is an alias for settings.Reasoning.
type Reasoning = s.Reasoning

// Web is an alias for settings.Web.
type Web = s.Web

// Discovery is an alias for settings.Discovery.
type Discovery = s.Discovery

// Sources is an alias for settings.Sources.
type Sources = s.Sources

// HookSpec is an alias for settings.HookSpec.
type HookSpec = s.HookSpec

type hookConfig []HookSpec

const ConfigFile = s.ConfigFile

func settings(flags Config) (Config, error) {
	return s.Settings(defaultSystem, flags)
}

func defaults() Config { return s.Defaults("") }

var derefBool = s.DerefBool
