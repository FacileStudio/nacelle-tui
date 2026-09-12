package settings

import "testing"

// deny_elevation rides the same linear chain as every other toggle, and both
// directions of an override have to work: a file's false must win over the on
// default, and the environment's true must win back over that file. A guard
// that could only ever tighten would not be the setting it claims to be. There
// is no CLI flag for it (there is none for path_isolation either), so the last
// layer is exercised through the Config the flags would fill.
func TestDenyElevationFallsThroughTheWholeChain(t *testing.T) {
	written(t, "")
	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if !*config.DenyElevation {
		t.Error("deny elevation = false, want it on by default")
	}

	written(t, "security:\n  deny_elevation: false\n")
	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if *config.DenyElevation {
		t.Error("deny elevation = true, want the file's false to win")
	}

	t.Setenv(EnvPrefix+"DENY_ELEVATION", "true")
	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if !*config.DenyElevation {
		t.Error("deny elevation = false, want the environment to win over the file")
	}

	off := false
	if config, err := settings(Config{Security: Security{DenyElevation: &off}}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if *config.DenyElevation {
		t.Error("deny elevation = true, want the flag layer to win over the environment")
	}
}
