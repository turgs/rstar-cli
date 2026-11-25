package commands

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/config"
	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/version"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass | fail | warn | skip
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local environment, auth, and API reachability",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		checks := []doctorCheck{}
		checks = append(checks, doctorCheck{
			Name:   "binary",
			Status: "pass",
			Detail: fmt.Sprintf("rstar %s (%s/%s)", version.Version, runtime.GOOS, runtime.GOARCH),
		})

		cfg, err := config.Load(config.ResolveOptions{
			ProfileName: flagProfile,
			BaseURL:     flagBaseURL,
			Token:       flagToken,
		})
		if err != nil {
			checks = append(checks, doctorCheck{Name: "config", Status: "fail", Detail: err.Error()})
			return finishDoctor(checks)
		}
		checks = append(checks, doctorCheck{
			Name:   "config",
			Status: "pass",
			Detail: fmt.Sprintf("dir=%s profile=%s", cfg.ConfigDir, cfg.ProfileName),
		})

		if cfg.BaseURL == "" {
			checks = append(checks, doctorCheck{
				Name:   "base_url",
				Status: "fail",
				Detail: "not set",
				Hint:   "rstar auth login --base-url https://YOUR.receptionstar.com TOKEN",
			})
		} else {
			checks = append(checks, doctorCheck{
				Name:   "base_url",
				Status: "pass",
				Detail: fmt.Sprintf("%s (from %s)", cfg.BaseURL, cfg.BaseURLSource),
			})
		}

		token := cfg.Token
		tokenSource := cfg.TokenSource
		if token == "" {
			tok, st, err := authStore.Get(cfg.ProfileName)
			if err != nil {
				checks = append(checks, doctorCheck{Name: "auth", Status: "fail", Detail: err.Error()})
				return finishDoctor(checks)
			}
			token, tokenSource = tok, st
		}
		if token == "" {
			checks = append(checks, doctorCheck{
				Name:   "auth",
				Status: "fail",
				Detail: "no token",
				Hint:   "rstar auth login --base-url https://YOUR.receptionstar.com TOKEN",
			})
		} else {
			checks = append(checks, doctorCheck{
				Name:   "auth",
				Status: "pass",
				Detail: fmt.Sprintf("token present (source=%s)", emptyDash(tokenSource)),
			})
		}

		if cfg.BaseURL != "" && token != "" {
			_, client, err := resolveRuntime()
			if err != nil {
				checks = append(checks, doctorCheck{Name: "api", Status: "fail", Detail: err.Error()})
			} else {
				client.Token = token
				client.BaseURL = cfg.BaseURL
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				res, err := client.Get(ctx, "/heart/authed", nil)
				if err != nil {
					ce := output.AsCLIError(err)
					checks = append(checks, doctorCheck{
						Name:   "api",
						Status: "fail",
						Detail: ce.Body.Message,
						Hint:   ce.Body.Hint,
					})
				} else {
					checks = append(checks, doctorCheck{
						Name:   "api",
						Status: "pass",
						Detail: fmt.Sprintf("GET /heart/authed => %d", res.StatusCode),
					})
				}
			}
		} else {
			checks = append(checks, doctorCheck{Name: "api", Status: "skip", Detail: "needs base URL and token"})
		}

		return finishDoctor(checks)
	},
}

func finishDoctor(checks []doctorCheck) error {
	passed, failed, warned, skipped := 0, 0, 0, 0
	for _, c := range checks {
		switch c.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "warn":
			warned++
		default:
			skipped++
		}
	}
	summary := fmt.Sprintf("%d passed, %d failed, %d warned, %d skipped", passed, failed, warned, skipped)
	data := map[string]any{
		"checks":  checks,
		"passed":  passed,
		"failed":  failed,
		"warned":  warned,
		"skipped": skipped,
	}
	if flagJSON {
		env := output.Envelope{OK: failed == 0, Data: data, Summary: summary}
		if failed > 0 {
			env.Error = &output.ErrorBody{Code: output.CodeConfig, Message: summary, Retryable: false}
		}
		if err := writeIndentedJSON(stdout, env); err != nil {
			return err
		}
		if failed > 0 {
			return &output.SilentExit{Code: output.ExitAPI}
		}
		return nil
	}
	fmt.Fprintf(stdout, "rstar doctor: %s\n", summary)
	for _, c := range checks {
		line := fmt.Sprintf("  [%s] %s", c.Status, c.Name)
		if c.Detail != "" {
			line += ": " + c.Detail
		}
		fmt.Fprintln(stdout, line)
		if c.Hint != "" {
			fmt.Fprintf(stdout, "         hint: %s\n", c.Hint)
		}
	}
	if failed > 0 {
		return &output.SilentExit{Code: output.ExitAPI}
	}
	return nil
}
