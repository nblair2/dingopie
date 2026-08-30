package cmd

import (
	"fmt"
	"runtime/debug"
)

// Set via -ldflags -X by .goreleaser.yml; left at defaults for `go build`/`go run`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionString falls back to Go's own VCS build-info stamping.
func versionString() string {
	v, c, d := version, commit, date

	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					c = s.Value
				case "vcs.time":
					d = s.Value
				case "vcs.modified":
					if s.Value == "true" {
						c += "-dirty"
					}
				}
			}
		}
	}

	return fmt.Sprintf("%s (commit %s, built %s)", v, c, d)
}

func init() {
	rootCmd.Version = versionString()
}
