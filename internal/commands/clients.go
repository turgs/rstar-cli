package commands

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
)

var (
	clientsPage    int
	clientsPerPage int
	clientsSearch  string
	clientsFields  string
)

var clientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Client records (P1 PHI - handle carefully)",
}

var clientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List clients",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{}
		if clientsPage > 0 {
			params["page"] = strconv.Itoa(clientsPage)
		}
		if clientsPerPage > 0 {
			params["per_page"] = strconv.Itoa(clientsPerPage)
		}
		if clientsSearch != "" {
			params["search"] = clientsSearch
		}
		if clientsFields != "" {
			params["fields"] = clientsFields
		}
		res, err := d.Call(ctx(), "clients", "list", params)
		if err != nil {
			return err
		}
		return printResult(res, output.Breadcrumb{
			Label: "show", Command: "rstar clients show ID --json", Hint: "Replace ID with a client id",
		})
	},
}

var clientsShowCmd = &cobra.Command{
	Use:   "show ID",
	Short: "Show one client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{"id": args[0]}
		if clientsFields != "" {
			params["fields"] = clientsFields
		}
		res, err := d.Call(ctx(), "clients", "show", params)
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

func init() {
	clientsListCmd.Flags().IntVar(&clientsPage, "page", 0, "Page number")
	clientsListCmd.Flags().IntVar(&clientsPerPage, "per-page", 0, "Page size (max 100)")
	clientsListCmd.Flags().StringVar(&clientsSearch, "search", "", "Search name/email")
	clientsListCmd.Flags().StringVar(&clientsFields, "fields", "", "Sparse fields")
	clientsShowCmd.Flags().StringVar(&clientsFields, "fields", "", "Sparse fields")
	clientsCmd.AddCommand(clientsListCmd, clientsShowCmd)
}
