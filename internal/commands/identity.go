package commands

import (
	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Identity and practice profile",
}

var identityWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated user for this practice",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		res, err := d.Call(ctx(), "identity", "whoami", nil)
		if err != nil {
			return err
		}
		return printResult(res, output.Breadcrumb{
			Label: "clients", Command: "rstar clients list --json", Hint: "List clients",
		})
	},
}

var identityPracticesCmd = &cobra.Command{
	Use:   "practices",
	Short: "Show the active CLI practice profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		res, err := d.Call(ctx(), "identity", "practices", nil)
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

func init() {
	identityCmd.AddCommand(identityWhoamiCmd, identityPracticesCmd)
}
