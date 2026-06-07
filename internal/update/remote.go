package update

import (
	"context"
	"os"

	"github.com/creativeprojects/go-selfupdate"
)

// repoSlug is the GitHub repository releases are fetched from.
const repoSlug = "StrangeNoob/speed-test-cli"

// newUpdater builds an updater that validates downloads against checksums.txt.
func newUpdater() (*selfupdate.Updater, error) {
	return selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
}

// Latest returns the newest release version tag (e.g. "v0.2.0"), or "" if none
// is found.
func Latest(ctx context.Context) (string, error) {
	up, err := newUpdater()
	if err != nil {
		return "", err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return "", err
	}
	if !found || rel == nil {
		return "", nil
	}
	return rel.Version(), nil
}

// Apply updates the running binary to the latest release if it is newer than
// current. It returns the new version on success, or ("", nil) when already up
// to date. The running executable is replaced atomically by go-selfupdate.
func Apply(ctx context.Context, current string) (string, error) {
	up, err := newUpdater()
	if err != nil {
		return "", err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return "", err
	}
	if !found || rel == nil {
		return "", nil
	}
	if upToDate(current, rel.Version()) {
		return "", nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := up.UpdateTo(ctx, rel, exe); err != nil {
		return "", err
	}
	return rel.Version(), nil
}
