// Command rstar is the ReceptionStar CLI.
package main

import (
	"os"

	"github.com/turgs/rstar-cli/internal/commands"
)

func main() {
	os.Exit(commands.Execute())
}
