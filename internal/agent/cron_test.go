package agent

import (
	"testing"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

func TestApplyJobDefaultsToNoShell(t *testing.T) {
	base := settings.Defaults("")
	tru := true

	job := settings.CronJob{
		Name:    "brief",
		Workdir: "~/x",
		Model:   "model-x",
	}
	cfg := applyJob(base, job)

	if *cfg.Bash != false {
		t.Errorf("commands not set defaults to bash=false, got true")
	}
	if *cfg.ApproveTools != false {
		t.Errorf("approval gate must be off for a scheduled run, got true")
	}
	if cfg.Root != expandHome("~/x") {
		t.Errorf("workdir not applied: got %q", cfg.Root)
	}
	if cfg.Model != "model-x" {
		t.Errorf("model not applied: got %q", cfg.Model)
	}
	if *cfg.Diffs != *base.Diffs {
		t.Errorf("unrelated toggle must be inherited untouched")
	}

	job.Commands = &tru
	if cfg2 := applyJob(base, job); *cfg2.Bash != true {
		t.Errorf("commands: true must enable bash, got false")
	}
}

func TestFindCronJob(t *testing.T) {
	config := settings.Config{
		Cron: []settings.CronJob{
			{Name: "one"},
			{Name: "two"},
		},
	}
	if job, err := findCronJob(config, "two"); err != nil || job.Name != "two" {
		t.Errorf("findCronJob(two) = %q, %v; want two, nil", job.Name, err)
	}
	if _, err := findCronJob(config, "missing"); err == nil {
		t.Errorf("findCronJob(missing) should error")
	}
}

func TestJobPointerDefaults(t *testing.T) {
	if jobEnabled(settings.CronJob{}) {
		t.Errorf("a job with no enabled setting must default to disabled")
	}
	if jobCommands(settings.CronJob{}) {
		t.Errorf("a job with no commands setting must default to shell-less")
	}
}
