package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

// cronStatus is the outcome appended to a job's delivery log.
type cronStatus string

const (
	cronOK     cronStatus = "ok"
	cronFailed cronStatus = "failed"
)

// checkCronFlag handles the `cron` subcommand, which fronts the headless run
// path so a scheduled job can be invoked from systemd or crontab with no TUI.
// It returns true when it recognised a cron command, and the caller should then
// treat its error as the process's result.
func checkCronFlag() (bool, error) {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "cron" {
		return false, nil
	}
	sub := args[1:]
	if len(sub) == 0 || sub[0] == "list" || sub[0] == "status" {
		return true, listCronJobs()
	}
	switch sub[0] {
	case "run":
		if len(sub) < 2 {
			return true, fmt.Errorf("usage: nacelle cron run <name>")
		}
		return true, runCronJob(sub[1])
	case "install":
		if len(sub) < 2 {
			return true, fmt.Errorf("usage: nacelle cron install <name>")
		}
		return true, installCronJob(sub[1])
	default:
		return true, fmt.Errorf("unknown cron command %q: want run, install, or list", sub[0])
	}
}

func loadCronConfig() (settings.Config, error) {
	flags := settings.FromFlags(settings.Defaults(""))
	return settings.Settings(DefaultSystemPrompt(), flags)
}

func findCronJob(config settings.Config, name string) (settings.CronJob, error) {
	for _, job := range config.Cron {
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
	text, runErr := runHeadlessConfig(job.Prompt, applyJob(config, job))
	status := cronOK
	if runErr != nil {
		status = cronFailed
	}
	logErr := appendCronLog(job.Delivery, job.Name, status, text)
	if runErr != nil {
		return runErr
	}
	return logErr
}

// appendCronLog delivers a run's outcome. An empty delivery target leaves the
// journal as the archive (the transcript already streams to stdout); a
// file:<dir> target appends the transcript and a status line to <dir>/<name>.log
// so the output lands somewhere greppable without a TUI.
func appendCronLog(delivery, name string, status cronStatus, text string) error {
	dir := ""
	if rest, ok := strings.CutPrefix(delivery, "file:"); ok {
		dir = rest
	} else if delivery != "" {
		return fmt.Errorf("unknown delivery %q: want file:<dir>", delivery)
	}
	if dir == "" {
		return nil
	}
	dir = expandHome(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating delivery dir %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, name+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s.log: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	header := fmt.Sprintf("\n=== %s %s ===\n", time.Now().Format(time.RFC3339), status)
	if _, err := f.WriteString(header + text); err != nil {
		return fmt.Errorf("writing %s.log: %w", name, err)
	}
	return nil
}
