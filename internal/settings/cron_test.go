package settings

import (
	"os"
	"testing"
)

// TestCronSurvivesSettingsResolution guards the rule that a cron job declared
// in the config file must reach the resolved Config. merge copies fields one
// by one, so a new top-level field has to be added there too or the file layer
// silently drops it.
func TestCronSurvivesSettingsResolution(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/.nacelle.yml"
	if err := os.WriteFile(configPath, []byte(`cron:
  - name: brief
    when: daily
    enabled: true
    prompt: "x"
`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	t.Setenv("HOME", dir)

	cfg, err := Settings("", Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if len(cfg.Automation.Cron) != 1 || cfg.Automation.Cron[0].Name != "brief" {
		t.Errorf("settings resolution lost the cron job: got %+v", cfg.Automation.Cron)
	}
}
