package agent

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
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

func TestValidateDelivery(t *testing.T) {
	for _, want := range []string{"", "file:~/logs", "file:"} {
		if err := validateDelivery(want); err != nil {
			t.Errorf("validateDelivery(%q) = %v, want nil", want, err)
		}
	}
	for _, bad := range []string{"webhook:x", "file", "~/logs"} {
		if err := validateDelivery(bad); err == nil {
			t.Errorf("validateDelivery(%q) passed, want a refusal of the unknown target", bad)
		}
	}
}

func TestHoistJSON(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"nacelle", "cron", "list", "--json"}
	if rest := hoistJSON([]string{"list", "--json"}); strings.Join(rest, " ") != "list" {
		t.Errorf("rest = %v, want the json token stripped", rest)
	}
	if got := strings.Join(os.Args, " "); got != "nacelle --json cron list" {
		t.Errorf("os.Args = %q, want the flag hoisted ahead of cron", got)
	}

	os.Args = []string{"nacelle", "cron", "list", "-json=false"}
	hoistJSON([]string{"list", "-json=false"})
	if got := strings.Join(os.Args, " "); got != "nacelle -json=false cron list" {
		t.Errorf("os.Args = %q, want -json=false hoisted too", got)
	}

	os.Args = []string{"nacelle", "cron", "run", "brief"}
	if rest := hoistJSON([]string{"run", "brief"}); strings.Join(rest, " ") != "run brief" {
		t.Errorf("rest = %v, want the args untouched", rest)
	}
	if got := strings.Join(os.Args, " "); got != "nacelle cron run brief" {
		t.Errorf("os.Args = %q, want it untouched when no -json is typed", got)
	}
}

func TestCronUsageErrors(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	for _, args := range [][]string{
		{"nacelle", "cron", "run"},
		{"nacelle", "cron", "install"},
		{"nacelle", "cron", "bogus"},
	} {
		os.Args = args
		handled, err := checkCronFlag()
		var usage *UsageError
		if !handled || err == nil || !errors.As(err, &usage) {
			t.Errorf("checkCronFlag(%v) = %t, %v; want a usage error", args, handled, err)
		}
	}
}

func TestCronJobsJSON(t *testing.T) {
	enabled, commands := true, false
	data, err := cronJobsJSON([]settings.CronJob{
		{Name: "brief", When: "daily", Enabled: &enabled, Commands: &commands, Workdir: "~/x", Delivery: "file:~/logs"},
	})
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	var jobs []cronJobJSON
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("the document is not the job list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	job := jobs[0]
	if job.Name != "brief" || job.When != "daily" || !job.Enabled || job.Commands || job.Workdir != "~/x" || job.Delivery != "file:~/logs" {
		t.Errorf("job json = %+v, want the pointer settings resolved to their defaults", job)
	}
	empty, err := cronJobsJSON(nil)
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	if string(empty) != "[]" {
		t.Errorf("an empty job set marshals to %s, want []", empty)
	}
}
