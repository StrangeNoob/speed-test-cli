// Command speed-test measures internet speed against Cloudflare.
package main

import "github.com/StrangeNoob/speed-test-cli/cmd"

// Injected at build time via -ldflags (see .goreleaser.yaml and the Makefile).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
