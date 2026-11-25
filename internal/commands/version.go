package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print binary version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := []string{"rstar", version.Version}
		if c := strings.TrimSpace(version.Commit); c != "" {
			parts = append(parts, "("+c+")")
		}
		line := strings.Join(parts, " ")
		if flagJSON {
			return output.WriteSuccess(stdout, map[string]any{
				"version": version.Version,
				"commit":  version.Commit,
			}, line)
		}
		fmt.Fprintln(stdout, line)
		return nil
	},
}
