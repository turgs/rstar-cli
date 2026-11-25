// Package version holds build-time identity for the rstar binary.
package version

// Version is the semantic version string. Overridden at link time in releases:
//
//	go build -ldflags "-X github.com/turgs/rstar-cli/internal/version.Version=1.2.3"
var Version = "0.0.0-dev"

// Commit is the short git SHA when set at link time; empty in plain local builds.
var Commit = ""
