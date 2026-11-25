package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/config"
	"github.com/turgs/rstar-cli/internal/output"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage practice authentication",
}

var authLoginBaseURL string
var authLoginProfile string

var authLoginCmd = &cobra.Command{
	Use:   "login TOKEN",
	Short: "Save a staff personal access token",
	Long: "Saves a ReceptionStar API personal access token (rs_pat_…) for a practice.\n\n" +
		"Create a token in Settings > Your Account > API Keys, then:\n\n" +
		"  rstar auth login --base-url https://YOUR.receptionstar.com TOKEN\n\n" +
		"The token is stored in the OS keyring when available, otherwise a 0600 file\n" +
		"under the rstar config directory.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := strings.TrimSpace(args[0])
		if token == "" {
			return output.ErrUsage("token is required")
		}
		base := firstNonEmpty(authLoginBaseURL, flagBaseURL)
		if base == "" {
			// allow existing profile base URL
			cfg, err := config.Load(config.ResolveOptions{ProfileName: firstNonEmpty(authLoginProfile, flagProfile)})
			if err != nil {
				return output.ErrConfig(err.Error())
			}
			base = cfg.BaseURL
		}
		if base == "" {
			return output.ErrUsage("base URL is required").WithHint("Pass --base-url https://YOUR.receptionstar.com")
		}
		profile := firstNonEmpty(authLoginProfile, flagProfile)
		if profile == "" {
			profile = "default"
		}
		if err := config.UpsertProfile(profile, base, true); err != nil {
			return output.ErrConfig(err.Error())
		}
		storage, err := authStore.Set(profile, token)
		if err != nil {
			return output.ErrConfig(err.Error())
		}
		data := map[string]any{
			"authenticated": true,
			"profile":       profile,
			"base_url":      config.NormalizeBaseURL(base),
			"storage":       storage,
		}
		crumbs := []output.Breadcrumb{
			{Label: "status", Command: "rstar auth status --json", Hint: "Check auth"},
			{Label: "whoami", Command: "rstar identity whoami --json", Hint: "Verify API access"},
			{Label: "doctor", Command: "rstar doctor --json", Hint: "Full health check"},
		}
		if flagJSON {
			return output.WriteSuccess(stdout, data, "token saved", crumbs...)
		}
		fmt.Fprintf(stdout, "Token saved for profile %q (%s) at %s\n", profile, storage, config.NormalizeBaseURL(base))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the saved token for the active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ResolveOptions{ProfileName: flagProfile})
		if err != nil {
			return output.ErrConfig(err.Error())
		}
		if err := authStore.Delete(cfg.ProfileName); err != nil {
			return output.ErrConfig(err.Error())
		}
		if flagJSON {
			return output.WriteSuccess(stdout, map[string]any{
				"authenticated": false,
				"profile":       cfg.ProfileName,
			}, "token removed")
		}
		fmt.Fprintf(stdout, "Token removed for profile %q\n", cfg.ProfileName)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status (never prints the token)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ResolveOptions{
			ProfileName: flagProfile,
			BaseURL:     flagBaseURL,
			Token:       flagToken,
		})
		if err != nil {
			return output.ErrConfig(err.Error())
		}
		storage := cfg.TokenSource
		hasToken := cfg.Token != ""
		if !hasToken {
			tok, st, err := authStore.Get(cfg.ProfileName)
			if err != nil {
				return output.ErrConfig(err.Error())
			}
			hasToken = tok != ""
			storage = st
		}
		data := map[string]any{
			"profile":       cfg.ProfileName,
			"base_url":      cfg.BaseURL,
			"authenticated": hasToken,
			"token_source":  storage,
			"base_source":   cfg.BaseURLSource,
		}
		summary := "not authenticated"
		if hasToken && cfg.BaseURL != "" {
			summary = "authenticated"
		} else if hasToken {
			summary = "token present; base URL missing"
		} else if cfg.BaseURL != "" {
			summary = "base URL set; token missing"
		}
		if flagJSON {
			return output.WriteSuccess(stdout, data, summary)
		}
		fmt.Fprintf(stdout, "profile: %s\n", cfg.ProfileName)
		fmt.Fprintf(stdout, "base_url: %s\n", emptyDash(cfg.BaseURL))
		fmt.Fprintf(stdout, "authenticated: %v\n", hasToken)
		if storage != "" {
			fmt.Fprintf(stdout, "token_source: %s\n", storage)
		}
		return nil
	},
}

func emptyDash(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func init() {
	authLoginCmd.Flags().StringVar(&authLoginBaseURL, "base-url", "", "Practice base URL")
	authLoginCmd.Flags().StringVar(&authLoginProfile, "profile", "", "Profile name (default: default)")
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
}
