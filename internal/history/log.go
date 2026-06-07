package history

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// Append writes the result as one JSON line to the file at path, creating
// parent directories and the file if needed.
func Append(path string, res speedtest.Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// DefaultPath returns the default history file location under the user's home.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".speed-test", "history.jsonl"), nil
}
