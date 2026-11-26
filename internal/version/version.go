// Package version says which build of Lithium is running and where updates
// come from.
package version

import "runtime/debug"

// Repo is the GitHub repository updates are checked against, and Branch the
// branch that install.sh pulls.
const (
	Repo   = "Ben-wh/Lithium"
	Branch = "main"
)

// commit is set at build time:
//
//	go build -ldflags "-X github.com/lithium/lithium/internal/version.commit=<sha>"
//
// The Dockerfile does this from the LITHIUM_COMMIT build argument, which
// install.sh fills in from git.
var commit string

// Commit returns the full git commit this binary was built from, or "" when
// it isn't known (e.g. `go run`, or `docker compose build` without
// LITHIUM_COMMIT set).
func Commit() string {
	if commit != "" {
		return commit
	}
	// A plain `go build` inside a git checkout records the revision itself.
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return ""
}
