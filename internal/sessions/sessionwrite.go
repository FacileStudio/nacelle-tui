package sessions

import (
	"os"
	"time"
)

func stamped() string {
	return time.Now().Format(time.RFC3339Nano)
}

func appendLine(path string, line []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}
