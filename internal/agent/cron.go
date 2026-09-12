package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

// checkCronFlag handles the `cron` subcommand, which fronts the headless run
// path so a scheduled job can be invoked from systemd or crontab with no TUI.
// It returns true when it recognised a cron command, and the caller should then
// treat its error as the process's result.
// findCronCommand scans os.Args for the cron subcommand: flags typed ahead of
// it stay where they are, where the settings flag parser reads them; the scan
// only skips leading dash-tokens, so a flag that takes a value must follow
// the subcommand instead.
func findCronCommand() (prefix, sub []string, ok bool) {
	args := os.Args[1:]
	first := 0
	for first < len(args) && strings.HasPrefix(args[first], "-") {
		first++
	}
	if first >= len(args) || args[first] != "cron" {
		return nil, nil, false
	}
	return args[:first], args[first+1:], true
}

func checkCronFlag() (bool, error) {
	prefix, sub, ok := findCronCommand()
	if !ok {
		return false, nil
	}
	sub = hoistJSON(prefix, sub)
	if len(sub) == 0 || sub[0] == "list" || sub[0] == "status" {
		return true, listCronJobs()
	}
	switch sub[0] {
	case "help", "-h", "--help":
		return true, printCronUsage()
	case "run":
		if len(sub) < 2 {
			return true, usagef("usage: nacelle cron run <name>")
		}
		return true, runCronJob(sub[1])
	case "install":
		if len(sub) < 2 {
			return true, usagef("usage: nacelle cron install <name>")
		}
		return true, installCronJob(sub[1])
	default:
		return true, usagef("unknown cron command %q: want run, install, or list", sub[0])
	}
}

// hoistJSON moves a trailing -json ahead of the subcommand: the settings flag
// parser stops at the first non-flag word, which "cron" always is, so a flag
// typed after the subcommand would otherwise be invisible to it. There is a
// precedent for rewriting os.Args mid-dispatch in extractPrintFlag.
func hoistJSON(prefix, sub []string) []string {
	rest := make([]string, 0, len(sub))
	json := ""
	for _, arg := range sub {
		if arg == "-json" || arg == "--json" || strings.HasPrefix(arg, "-json=") || strings.HasPrefix(arg, "--json=") {
			json = arg
			continue
		}
		rest = append(rest, arg)
	}
	if json != "" {
		os.Args = append(append([]string{os.Args[0]}, prefix...), append([]string{json, "cron"}, rest...)...)
	}
	return rest
}

func loadCronConfig() (settings.Config, error) {
	flags := settings.FromFlags(settings.Defaults(""))
	return settings.Settings(DefaultSystemPrompt(), flags)
}

func findCronJob(config settings.Config, name string) (settings.CronJob, error) {
	for _, job := range config.Automation.Cron {
		if job.Name == name {
			return job, nil
		}
	}
	return settings.CronJob{}, fmt.Errorf("no cron job named %q", name)
}

// applyJob projects a job's overrides onto the resolved session config. The
// two autonomy fields deliberately invert the interactive defaults: a run no
// one is watching cannot answer an approval prompt, so commands default to off
// and the approval gate is never armed. A job opts into shell with
// commands: true.
func applyJob(config settings.Config, job settings.CronJob) settings.Config {
	cfg := config
	if job.Workdir != "" {
		cfg.Root = expandHome(job.Workdir)
	}
	if job.Model != "" {
		cfg.Model = job.Model
	}
	bash := false
	if job.Commands != nil {
		bash = *job.Commands
	}
	cfg.Bash = &bash
	approve := false
	cfg.ApproveTools = &approve
	return cfg
}

func runCronJob(name string) error {
	config, err := loadCronConfig()
	if err != nil {
		return err
	}
	job, err := findCronJob(config, name)
	if err != nil {
		return err
	}
	if err := validateDelivery(job.Delivery); err != nil {
		return err
	}
	text, runErr := runHeadlessConfig(job.Prompt, applyJob(config, job))
	status := cronOK
	if runErr != nil {
		status = cronFailed
	}
	logErr := appendCronLog(job.Delivery, job.Name, status, text)
	return errors.Join(runErr, logErr)
}
