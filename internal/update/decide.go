// Package update checks GitHub for newer releases and can replace the running
// binary in place. All policy here is pure and offline; network/replace work
// lives in remote.go.
package update

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ShouldCheck reports whether the passive update check should run. It is skipped
// for machine output, when explicitly disabled, and for unversioned dev builds.
func ShouldCheck(jsonMode, noFlag bool, env, version string) bool {
	if jsonMode || noFlag || env != "" || version == "dev" {
		return false
	}
	return true
}

// ShouldPrompt reports whether to interactively prompt (only on a TTY).
func ShouldPrompt(isTTY bool) bool { return isTTY }

// PromptYesNo writes prompt to w and reads a yes/no answer from r. Empty or
// anything other than y/yes (case-insensitive) is false.
func PromptYesNo(r io.Reader, w io.Writer, prompt string) (bool, error) {
	fmt.Fprint(w, prompt)
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
