package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/nacelle-tui/internal/settings"
)

// listCronJobs prints the configured jobs and how to run or arm them. It arms
// nothing.
func listCronJobs() error {
	config, err := loadCronConfig()
	if err != nil {
		return err
	}
	if len(config.Cron) == 0 {
		fmt.Println("no cron jobs in " + settings.ConfigPath())
		return nil
	}
	for _, job := range config.Cron {
		fmt.Printf("%-20s when=%-18s enabled=%t commands=%t workdir=%s delivery=%s\n",
			job.Name, job.When, jobEnabled(job), jobCommands(job), job.Workdir, job.Delivery)
	}
	fmt.Println("\nrun one now:        nacelle cron run <name>")
	fmt.Println("arm its timer:      nacelle cron install <name>")
	return nil
}

// installCronJob prints the systemd service + timer pair that arm one job, for
// the user to drop under ~/.config/systemd/user/ (or /etc/systemd/system/ for a
// system timer). It refuses a disabled job: the policy is a passing test run
// and an explicit enabled: true before a schedule fires on its own.
func installCronJob(name string) error {
	config, err := loadCronConfig()
	if err != nil {
		return err
	}
	job, err := findCronJob(config, name)
	if err != nil {
		return err
	}
	if !jobEnabled(job) {
		return fmt.Errorf("job %q is disabled: run `nacelle cron run %s`, confirm the output, then set enabled: true",
			name, name)
	}
	schedule := job.When
	if schedule == "" {
		schedule = "daily"
	}
	timeout := job.Timeout
	if timeout == "" {
		timeout = "300"
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating nacelle: %w", err)
	}
	svc, timer := cronUnits(job.Name, bin, schedule, timeout)
	fmt.Println(svc)
	fmt.Println(timer)
	fmt.Printf("# save both files, then: systemctl --user enable --now nacelle-%s.timer\n", job.Name)
	return nil
}

func cronUnits(name, bin, schedule, timeout string) (service, timer string) {
	service = fmt.Sprintf(`[Unit]
Description=nacelle cron %[1]s

[Service]
Type=oneshot
ExecStart=%[2]s cron run %[1]s
TimeoutStopSec=%[3]s

[Install]
WantedBy=default.target
`, name, bin, timeout)
	timer = fmt.Sprintf(`[Unit]
Description=schedule for nacelle cron %[1]s

[Timer]
OnCalendar=%[4]s
Unit=nacelle-%[1]s.service
Persistent=false

[Install]
WantedBy=timers.target
`, name, bin, timeout, schedule)
	return service, timer
}

// jobEnabled and jobCommands read a job's pointer settings with the policy
// defaults filled in: a job is disabled and shell-less until it says otherwise.
func jobEnabled(job settings.CronJob) bool {
	return job.Enabled != nil && *job.Enabled
}

func jobCommands(job settings.CronJob) bool {
	return job.Commands != nil && *job.Commands
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
