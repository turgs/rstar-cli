package commands

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/catalog"
	"github.com/turgs/rstar-cli/internal/mcpserver"
	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/version"
)

var (
	mcpWrites  bool
	mcpDomains []string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve ReceptionStar to MCP clients over stdio",
	Long: "Run an MCP server on stdin/stdout using the shared domain catalog.\n\n" +
		"Read-only by default. Opt in to writes with --writes (pair with a token that\n" +
		"has write scopes). Narrow domains with --domains (fail-closed on unknown names).\n\n" +
		"  claude mcp add rstar -- rstar mcp\n" +
		"  claude mcp add rstar -- rstar mcp --domains identity,clients\n" +
		"  claude mcp add rstar -- rstar mcp --writes\n\n" +
		"Does not exec CLI subprocesses; dispatch uses the in-process HTTP client.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, client, err := requireClient()
		if err != nil {
			return err
		}
		// stdout is the MCP wire - log to stderr only.
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		err = mcpserver.RunStdio(ctx, client, catalog.ProfileInfo{
			ProfileName: cfg.ProfileName,
			BaseURL:     cfg.BaseURL,
		}, mcpserver.Config{
			ReadOnly: !mcpWrites,
			Domains:  mcpDomains,
			Version:  version.Version,
		}, logger)
		if err != nil {
			return output.NewError(output.CodeAPI, err.Error(), output.ExitAPI)
		}
		return nil
	},
}

func init() {
	mcpCmd.Flags().BoolVar(&mcpWrites, "writes", false, "Serve write actions too (pair with a write-capable PAT)")
	mcpCmd.Flags().StringSliceVar(&mcpDomains, "domains", nil, "Comma-separated domain keys (default all; unknown names fail closed)")
}
