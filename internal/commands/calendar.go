package commands

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
)

var (
	calendarPage          int
	calendarPerPage       int
	calendarFromDate      string
	calendarToDate        string
	calendarIsAppointment string
	calendarClientID      string
	calendarClientIDs     string
	calendarUserID        string
	calendarUserIDs       string
	calendarLocationID    string
	calendarLocationIDs   string
	calendarSort          string
	calendarTypesPage     int
	calendarTypesPerPage  int
)

var calendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "Appointments and appointment types",
}

var calendarListCmd = &cobra.Command{
	Use:   "list",
	Short: "List appointments",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{}
		if calendarPage > 0 {
			params["page"] = strconv.Itoa(calendarPage)
		}
		if calendarPerPage > 0 {
			params["per_page"] = strconv.Itoa(calendarPerPage)
		}
		if calendarFromDate != "" {
			params["from_date"] = calendarFromDate
		}
		if calendarToDate != "" {
			params["to_date"] = calendarToDate
		}
		if calendarIsAppointment != "" {
			params["is_appointment"] = calendarIsAppointment
		}
		if calendarClientID != "" {
			params["client_id"] = calendarClientID
		}
		if calendarClientIDs != "" {
			params["client_ids"] = calendarClientIDs
		}
		if calendarUserID != "" {
			params["user_id"] = calendarUserID
		}
		if calendarUserIDs != "" {
			params["user_ids"] = calendarUserIDs
		}
		if calendarLocationID != "" {
			params["location_id"] = calendarLocationID
		}
		if calendarLocationIDs != "" {
			params["location_ids"] = calendarLocationIDs
		}
		if calendarSort != "" {
			params["sort"] = calendarSort
		}
		res, err := d.Call(ctx(), "calendar", "list", params)
		if err != nil {
			return err
		}
		return printResult(res, output.Breadcrumb{
			Label: "show", Command: "rstar calendar show ID --json", Hint: "Replace ID with an engagement id",
		})
	},
}

var calendarShowCmd = &cobra.Command{
	Use:   "show ID",
	Short: "Show one appointment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		res, err := d.Call(ctx(), "calendar", "show", map[string]any{"id": args[0]})
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

var calendarTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List appointment types",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, _, err := dispatcher(false)
		if err != nil {
			return err
		}
		params := map[string]any{}
		if calendarTypesPage > 0 {
			params["page"] = strconv.Itoa(calendarTypesPage)
		}
		if calendarTypesPerPage > 0 {
			params["per_page"] = strconv.Itoa(calendarTypesPerPage)
		}
		res, err := d.Call(ctx(), "calendar", "list_appointment_types", params)
		if err != nil {
			return err
		}
		return printResult(res)
	},
}

func init() {
	calendarListCmd.Flags().IntVar(&calendarPage, "page", 0, "Page number")
	calendarListCmd.Flags().IntVar(&calendarPerPage, "per-page", 0, "Page size (max 100)")
	calendarListCmd.Flags().StringVar(&calendarFromDate, "from-date", "", "From date YYYY-MM-DD")
	calendarListCmd.Flags().StringVar(&calendarToDate, "to-date", "", "To date YYYY-MM-DD")
	calendarListCmd.Flags().StringVar(&calendarIsAppointment, "is-appointment", "", "true|false (default true)")
	calendarListCmd.Flags().StringVar(&calendarClientID, "client-id", "", "Client id")
	calendarListCmd.Flags().StringVar(&calendarClientIDs, "client-ids", "", "Comma-separated client ids")
	calendarListCmd.Flags().StringVar(&calendarUserID, "user-id", "", "Practitioner id")
	calendarListCmd.Flags().StringVar(&calendarUserIDs, "user-ids", "", "Comma-separated practitioner ids")
	calendarListCmd.Flags().StringVar(&calendarLocationID, "location-id", "", "Location id")
	calendarListCmd.Flags().StringVar(&calendarLocationIDs, "location-ids", "", "Comma-separated location ids")
	calendarListCmd.Flags().StringVar(&calendarSort, "sort", "", "Sort e.g. start_date:asc")

	calendarTypesCmd.Flags().IntVar(&calendarTypesPage, "page", 0, "Page number")
	calendarTypesCmd.Flags().IntVar(&calendarTypesPerPage, "per-page", 0, "Page size (max 100)")

	calendarCmd.AddCommand(calendarListCmd, calendarShowCmd, calendarTypesCmd)
}
