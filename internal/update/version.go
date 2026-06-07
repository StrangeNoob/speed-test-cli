package update

import "golang.org/x/mod/semver"

// canon ensures a leading "v" so golang.org/x/mod/semver can parse it.
func canon(v string) string {
	if v == "" || v[0] == 'v' {
		return v
	}
	return "v" + v
}

// Newer reports whether latest is strictly newer than current. It returns false
// for a "dev" build or any unparseable version (used by the passive notice).
func Newer(current, latest string) bool {
	if current == "dev" {
		return false
	}
	c, l := canon(current), canon(latest)
	if !semver.IsValid(c) || !semver.IsValid(l) {
		return false
	}
	return semver.Compare(l, c) > 0
}

// upToDate reports whether current is a valid version that is >= latest. A
// dev/unparseable current is treated as NOT up to date so an explicit `update`
// still upgrades it.
func upToDate(current, latest string) bool {
	c, l := canon(current), canon(latest)
	if !semver.IsValid(c) {
		return false
	}
	if !semver.IsValid(l) {
		return true
	}
	return semver.Compare(c, l) >= 0
}
