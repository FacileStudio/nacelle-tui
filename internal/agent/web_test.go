package agent

import (
	"strings"
	"testing"
)

func TestFetchIsOnByDefaultAndTheFileCanTurnItOff(t *testing.T) {
	written(t, "")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Fetch {
		t.Error("fetch = false, want reading a page to be available without asking")
	}

	written(t, "fetch: false\n")
	if config, err = resolveSettings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Fetch {
		t.Error("fetch = true, want the file able to turn it off")
	}
}

func TestTheBannerNamesFetchOnlyWhenItIsOff(t *testing.T) {
	on, off := true, false

	quiet := testBanner(&answeringStub{}, asSettled(Config{Root: ".", Web: Web{Fetch: &on}}), loaded{}, connected{})
	if strings.Contains(quiet, "fetch") {
		t.Errorf("banner = %q, want nothing said about fetch when it is on", quiet)
	}

	loud := testBanner(&answeringStub{}, asSettled(Config{Root: ".", Web: Web{Fetch: &off}}), loaded{}, connected{})
	if !strings.Contains(loud, "fetch off") {
		t.Errorf("banner = %q, want the reason a page cannot be read on screen", loud)
	}
}
