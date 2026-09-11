package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cronStatus is the outcome appended to a job's delivery log.
type cronStatus string

const (
	cronOK     cronStatus = "ok"
	cronFailed cronStatus = "failed"
)

// validateDelivery checks a job's delivery target before any billed run, so a
// bad delivery: value fails fast instead of after the full run.
func validateDelivery(delivery string) error {
	if delivery == "" || strings.HasPrefix(delivery, "file:") {
		return nil
	}
	return fmt.Errorf("unknown delivery %q: want file:<dir>", delivery)
}

func appendCronLog(delivery, name string, status cronStatus, text string) error {
	if err := validateDelivery(delivery); err != nil {
		return err
	}
	dir := ""
	if rest, ok := strings.CutPrefix(delivery, "file:"); ok {
		dir = rest
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
