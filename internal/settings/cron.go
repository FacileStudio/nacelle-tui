package settings

// CronJob is one scheduled background run. The external scheduler
// (systemd/crontab) owns the clock and fires `nacelle cron run <name>`;
// nacelle only generates the units and interprets the schedule for display.
// Commands and Enabled default to off, the reverse of the interactive
// defaults, because a run no one can approve starts shell-less and disarmed.
type CronJob struct {
	Name     string `yaml:"name"`
	When     string `yaml:"when"`
	Prompt   string `yaml:"prompt"`
	Workdir  string `yaml:"workdir"`
	Model    string `yaml:"model"`
	Delivery string `yaml:"delivery"`
	Timeout  string `yaml:"timeout"`
	Commands *bool  `yaml:"commands"`
	Enabled  *bool  `yaml:"enabled"`
}
