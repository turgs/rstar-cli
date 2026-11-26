// Package commands implements the rstar Cobra command tree.
package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/auth"
	"github.com/turgs/rstar-cli/internal/catalog"
	"github.com/turgs/rstar-cli/internal/config"
	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/version"
)

// Global flags / runtime state.
var (
	flagJSON       bool
	flagAgentHelp  bool
	flagProfile    string
	flagBaseURL    string
	flagToken      string
	flagAllowWrite bool

	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr

	authStore *auth.Store = auth.Default()
)

// rootCmd is the top-level command.
var rootCmd = &cobra.Command{
	Use:           "rstar",
	Short:         "ReceptionStar CLI",
	Long:          "Command-line tool for ReceptionStar - terminal use and coding agents.\n\nhttps://receptionstar.com",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	err := rootCmd.Execute()
	if err == nil {
		return output.ExitOK
	}
	if output.AlreadyPrinted(err) {
		return output.ExitCodeFor(err)
	}
	ce := output.AsCLIError(err)
	if flagJSON || wantsJSON(rootCmd) {
		_ = output.WriteError(stdout, ce)
	} else {
		fmt.Fprintf(stderr, "rstar: %s\n", ce.Body.Message)
		if ce.Body.Hint != "" {
			fmt.Fprintf(stderr, "hint: %s\n", ce.Body.Hint)
		}
	}
	return ce.ExitCode
}

func wantsJSON(cmd *cobra.Command) bool {
	if flagJSON {
		return true
	}
	// mcp and long-running servers stay off the envelope path.
	return false
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Emit the agent JSON envelope on stdout")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Config profile name")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "", "Practice base URL (overrides profile)")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "PAT (overrides keyring; prefer auth login)")
	rootCmd.PersistentFlags().BoolVar(&flagAllowWrite, "write", false, "Allow write actions on leaf commands that mutate data")
	// Persistent so `rstar --help --agent` works (ticket acceptance), not only `rstar help --agent`.
	rootCmd.PersistentFlags().BoolVar(&flagAgentHelp, "agent", false, "With help/--help: print agent-oriented help (discovery, envelope, PHI)")

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if flagAgentHelp || agentHelpRequested(os.Args) {
			printAgentHelp(cmd.OutOrStdout())
			return
		}
		defaultHelp(cmd, args)
	})

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(identityCmd)
	rootCmd.AddCommand(clientsCmd)
	rootCmd.AddCommand(calendarCmd)
	rootCmd.AddCommand(billingCmd)
	rootCmd.AddCommand(commandsCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(newHelpCommand())
}

func newHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagAgentHelp || agentHelpRequested(os.Args) {
				printAgentHelp(stdout)
				return nil
			}
			if len(args) == 0 {
				return rootCmd.Help()
			}
			target, _, err := rootCmd.Find(args)
			if err != nil {
				return output.ErrUsage(err.Error())
			}
			return target.Help()
		},
	}
}

func agentHelpRequested(args []string) bool {
	for _, a := range args {
		if a == "--agent" {
			return true
		}
	}
	return false
}

func printAgentHelp(w io.Writer) {
	fmt.Fprint(w, `rstar - agent help

Discovery
  rstar commands --json          Full command catalog
  rstar help --agent             This text
  rstar doctor --json            Config/auth/API health
  rstar upgrade                  Explicit self-update (release installs)
  rstar mcp                      Stdio MCP (read-only default)

Envelope (--json)
  Success: { "ok": true, "data": …, "summary": "…", "breadcrumbs": […] }
  Error:   { "ok": false, "error": { "code", "message", "retryable", "hint?" } }

Auth
  rstar auth login --base-url https://PRACTICE.receptionstar.com TOKEN
  Token from Settings > Your Account > API Keys (Bearer rs_pat_…).
  Stored in OS keyring when available; file fallback under the config dir.

Live /api/v1 slice
  rstar identity whoami --json
  rstar clients list --json
  rstar clients show ID --json
  rstar calendar list --from-date YYYY-MM-DD --json
  rstar calendar show ID --json
  rstar calendar types --json
  rstar billing invoices --json
  rstar billing invoice ID --json
  rstar billing unbilled --json

MCP domain gateways (ADR 0001)
  One tool per domain; call { "action", "params" }.
  In-band describe on every tool. Read-only default; --writes to opt in.
  --domains identity,clients,calendar,billing fails closed on unknown names.
  Catalog module: internal/catalog (shared spine for hosted MCP).

PHI
  Client show/list is P1 health data. Do not paste into public tickets/logs.
  Invoice show is P2; prefer --fields / view=summary when summarizing.
  Clinical note bodies are not in this slice; when they land they stay gated.

https://receptionstar.com
`)
}

// resolveRuntime loads config + token into a ready API client.
func resolveRuntime() (*config.Config, *apiclient.Client, error) {
	cfg, err := config.Load(config.ResolveOptions{
		ProfileName: flagProfile,
		BaseURL:     flagBaseURL,
		Token:       flagToken,
	})
	if err != nil {
		return nil, nil, output.ErrConfig(err.Error())
	}
	if cfg.BaseURL != "" {
		if err := config.ValidateBaseURL(cfg.BaseURL); err != nil {
			return nil, nil, output.ErrConfig(err.Error()).WithHint("Pass --base-url https://YOUR.receptionstar.com or re-run 'rstar auth login'.")
		}
	}
	if cfg.Token == "" {
		tok, _, err := authStore.Get(cfg.ProfileName)
		if err != nil {
			return nil, nil, output.ErrConfig(err.Error())
		}
		cfg.Token = tok
		if tok != "" {
			cfg.TokenSource = "store"
		}
	}
	client := apiclient.New(cfg.BaseURL, cfg.Token)
	if v := version.Version; v != "" {
		client.UserAgent = "rstar-cli/" + v
	}
	return cfg, client, nil
}

func requireClient() (*config.Config, *apiclient.Client, error) {
	cfg, client, err := resolveRuntime()
	if err != nil {
		return nil, nil, err
	}
	if cfg.BaseURL == "" {
		return nil, nil, output.ErrConfig("practice base URL is not set")
	}
	if cfg.Token == "" {
		return nil, nil, output.ErrAuth("not authenticated")
	}
	return cfg, client, nil
}

func dispatcher(allowWrite bool) (*catalog.Dispatcher, *config.Config, error) {
	cfg, client, err := requireClient()
	if err != nil {
		return nil, nil, err
	}
	return &catalog.Dispatcher{
		Client: client,
		Profile: catalog.ProfileInfo{
			ProfileName: cfg.ProfileName,
			BaseURL:     cfg.BaseURL,
		},
		AllowWrites: allowWrite || flagAllowWrite,
	}, cfg, nil
}

func printResult(res *catalog.Result, crumbs ...output.Breadcrumb) error {
	if flagJSON {
		env := output.Envelope{
			OK:          true,
			Data:        res.Data,
			Summary:     res.Summary,
			Breadcrumbs: crumbs,
			Meta:        res.Meta,
		}
		enc := jsonEncoder(stdout)
		return enc(env)
	}
	if res.Summary != "" {
		fmt.Fprintln(stdout, res.Summary)
	}
	enc := jsonEncoder(stdout)
	return enc(res.Data)
}

func jsonEncoder(w io.Writer) func(any) error {
	return func(v any) error {
		return outputWriteJSON(w, v)
	}
}

func outputWriteJSON(w io.Writer, v any) error {
	// reuse indent encoder via success path for raw data when human mode prints data
	return writeIndentedJSON(w, v)
}

func writeIndentedJSON(w io.Writer, v any) error {
	return output.WriteRawJSON(w, v)
}

// ctx returns a background context (hook for tests later).
func ctx() context.Context {
	return context.Background()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
