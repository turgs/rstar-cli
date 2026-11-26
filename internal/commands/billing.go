package commands

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
)

var (
	billingPage     int
	billingPerPage  int
	billingSearch   string
	billingStatus   string
	billingClientID string
	billingFields   string
	billingView     string
	unbilledPage    int
	unbilledPerPage int
)

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Invoices and unbilled appointments",
}

var billingInvoicesListCmd = &cobra.Command{
	Use:   "invoices",
	Short: "List invoices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{}
		if billingPage > 0 {
			params["page"] = strconv.Itoa(billingPage)
		}
		if billingPerPage > 0 {
			params["per_page"] = strconv.Itoa(billingPerPage)
		}
		if billingSearch != "" {
			params["search"] = billingSearch
		}
		if billingStatus != "" {
			params["status"] = billingStatus
		}
		if billingClientID != "" {
			params["client_id"] = billingClientID
		}
		if billingFields != "" {
			params["fields"] = billingFields
		}
		res, err := d.Call(ctx(), "billing", "list_invoices", params)
		if err != nil {
			return err
		}
		return printResult(res, output.Breadcrumb{
			Label: "show", Command: "rstar billing invoice ID --json", Hint: "Replace ID with an invoice id",
		})
	},
}

var billingInvoiceShowCmd = &cobra.Command{
	Use:   "invoice ID",
	Short: "Show one invoice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{"id": args[0]}
		if billingFields != "" {
			params["fields"] = billingFields
		}
		if billingView != "" {
			params["view"] = billingView
		}
		res, err := d.Call(ctx(), "billing", "show_invoice", params)
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

var billingUnbilledCmd = &cobra.Command{
	Use:   "unbilled",
	Short: "List clients with eligible unbilled past appointments",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{}
		if unbilledPage > 0 {
			params["page"] = strconv.Itoa(unbilledPage)
		}
		if unbilledPerPage > 0 {
			params["per_page"] = strconv.Itoa(unbilledPerPage)
		}
		res, err := d.Call(ctx(), "billing", "list_unbilled_appointments", params)
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

func init() {
	billingInvoicesListCmd.Flags().IntVar(&billingPage, "page", 0, "Page number")
	billingInvoicesListCmd.Flags().IntVar(&billingPerPage, "per-page", 0, "Page size (max 100)")
	billingInvoicesListCmd.Flags().StringVar(&billingSearch, "search", "", "Search term")
	billingInvoicesListCmd.Flags().StringVar(&billingStatus, "status", "", "Invoice status id")
	billingInvoicesListCmd.Flags().StringVar(&billingClientID, "client-id", "", "Client id")
	billingInvoicesListCmd.Flags().StringVar(&billingFields, "fields", "", "Sparse fields")

	billingInvoiceShowCmd.Flags().StringVar(&billingFields, "fields", "", "Sparse fields")
	billingInvoiceShowCmd.Flags().StringVar(&billingView, "view", "", "summary omits line items/payments")

	billingUnbilledCmd.Flags().IntVar(&unbilledPage, "page", 0, "Page number")
	billingUnbilledCmd.Flags().IntVar(&unbilledPerPage, "per-page", 0, "Page size (max 100)")

	billingCmd.AddCommand(billingInvoicesListCmd, billingInvoiceShowCmd, billingUnbilledCmd)
}
