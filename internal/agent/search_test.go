package agent

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

func TestSearchDefaultsToFuret(t *testing.T) {
	written(t, "")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "https://furet.facile.studio" {
		t.Errorf("search = %q, want https://furet.facile.studio by default", *config.Search)
	}
}

func TestSearchComesFromTheFileAndEveryLayerAboveOutranksIt(t *testing.T) {
	written(t, "search: https://from-the-file.example\n")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "https://from-the-file.example" {
		t.Errorf("search = %q, want the file's instance", *config.Search)
	}

	t.Setenv(settings.EnvPrefix+"SEARCH", "https://from-the-environment.example")
	if config, err = resolveSettings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "https://from-the-environment.example" {
		t.Errorf("search = %q, want the environment to win", *config.Search)
	}

	if config, err = resolveSettings(Config{Web: Web{Search: ptr("https://from-the-flag.example")}}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "https://from-the-flag.example" {
		t.Errorf("search = %q, want the flag to win", *config.Search)
	}
}

func TestAnUnusableSearchEndpointStopsTheClient(t *testing.T) {
	config := defaults()
	config.Root = t.TempDir()
	config.Search = ptr("furet.example/search")

	set, _, err := localTools(config)
	if set != nil {
		t.Cleanup(func() {
			set.Close()
		})
	}
	if err == nil {
		t.Fatal("localTools accepted an endpoint with no scheme, want a refusal before the client starts")
	}
	if !strings.Contains(err.Error(), "furet.example") {
		t.Errorf("error = %q, want it to quote the endpoint that is wrong", err)
	}
}

func TestSearchCanBeTurnedOffForOneRunByAnEmptyFlag(t *testing.T) {
	written(t, "search: https://from-the-file.example\n")

	config, err := resolveSettings(Config{Web: Web{Search: ptr("")}})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "" {
		t.Errorf("search = %q, want an explicitly empty flag to turn it off", *config.Search)
	}
}

func TestAnAbsentSearchVariableFallsThroughButAnEmptyOneOverrides(t *testing.T) {
	written(t, "search: https://from-the-file.example\n")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "https://from-the-file.example" {
		t.Errorf("search = %q, want an unset variable to leave the file alone", *config.Search)
	}

	t.Setenv(settings.EnvPrefix+"SEARCH", "")
	if config, err = resolveSettings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Search != "" {
		t.Errorf("search = %q, want an empty variable to turn it off", *config.Search)
	}
}

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
