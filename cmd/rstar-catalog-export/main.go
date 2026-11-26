// Export config/mcp/catalog.json shape for hosted ReceptionStar MCP.
// stdout = JSON only. Board: turgs/rstar-issues#508 (producer path), #506 (pin shape).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/turgs/rstar-cli/internal/catalog"
)

// snapshot uses a struct so json.MarshalIndent keeps stable key order.
type snapshot struct {
	Source         string           `json:"source"`
	RstarCLICommit string           `json:"rstar_cli_commit"`
	DomainsSHA256  string           `json:"domains_sha256"`
	Sync           string           `json:"sync"`
	Domains        []catalog.Domain `json:"domains"`
}

func main() {
	sha := "unknown"
	if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		sha = strings.TrimSpace(string(out))
	}

	domains := catalog.All()
	domJSON, err := json.Marshal(domains)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sum := sha256.Sum256(domJSON)

	payload := snapshot{
		Source:         "turgs/rstar-cli/internal/catalog",
		RstarCLICommit: sha,
		DomainsSHA256:  hex.EncodeToString(sum[:]),
		// Keep string stable so ReceptionStar bin/sync-mcp-catalog --check matches.
		Sync:    "bin/sync-mcp-catalog",
		Domains: domains,
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(append(b, '\n'))
}
