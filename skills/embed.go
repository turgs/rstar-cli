// Package skills embeds the agent skill markdown shipped with the CLI.
package skills

import _ "embed"

// Markdown is the rstar agent skill (discovery-first, PHI rules).
//
//go:embed rstar/SKILL.md
var Markdown string
