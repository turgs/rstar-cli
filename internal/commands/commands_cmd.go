package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/turgs/rstar-cli/internal/catalog"
	"github.com/turgs/rstar-cli/internal/output"
)

type commandInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Flags       []flagInfo    `json:"flags,omitempty"`
	Subcommands []commandInfo `json:"subcommands,omitempty"`
}

type flagInfo struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

var commandsCmd = &cobra.Command{
	Use:     "commands [filter]",
	Aliases: []string{"cmds"},
	Short:   "List commands (use --json for agents)",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := ""
		if len(args) == 1 {
			filter = args[0]
		}
		tree := walkCommands(rootCmd, "rstar", filter)
		domains := catalog.All()
		domainSummaries := make([]map[string]any, 0, len(domains))
		for _, d := range domains {
			implemented := 0
			for _, a := range d.Actions {
				if a.Implemented {
					implemented++
				}
			}
			domainSummaries = append(domainSummaries, map[string]any{
				"key":         d.Key,
				"tool":        d.Tool,
				"implemented": implemented,
				"actions":     len(d.Actions),
			})
		}
		data := map[string]any{
			"commands": tree,
			"catalog":  domainSummaries,
		}
		if flagJSON {
			return output.WriteSuccess(stdout, data, fmt.Sprintf("%d top-level commands", len(tree)))
		}
		for _, c := range tree {
			printCommandHuman(c, 0)
		}
		fmt.Fprintln(stdout, "\nMCP catalog domains:")
		for _, d := range domainSummaries {
			fmt.Fprintf(stdout, "  %s (%s) - %v/%v actions implemented\n", d["key"], d["tool"], d["implemented"], d["actions"])
		}
		return nil
	},
}

func printCommandHuman(c commandInfo, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(stdout, "%s%s - %s\n", indent, c.Name, c.Description)
	for _, sub := range c.Subcommands {
		printCommandHuman(sub, depth+1)
	}
}

func walkCommands(cmd *cobra.Command, prefix, filter string) []commandInfo {
	var result []commandInfo
	subs := cmd.Commands()
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
	for _, sub := range subs {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		full := prefix + " " + sub.Name()
		if filter != "" && !strings.Contains(full, filter) && !strings.Contains(sub.Name(), filter) {
			// still walk children for nested matches
			children := walkCommands(sub, full, filter)
			if len(children) == 0 {
				continue
			}
			result = append(result, commandInfo{
				Name:        full,
				Description: sub.Short,
				Flags:       collectFlags(sub),
				Subcommands: children,
			})
			continue
		}
		info := commandInfo{
			Name:        full,
			Description: sub.Short,
			Flags:       collectFlags(sub),
			Subcommands: walkCommands(sub, full, filter),
		}
		if filter == "" || strings.Contains(full, filter) || strings.Contains(sub.Name(), filter) || len(info.Subcommands) > 0 {
			result = append(result, info)
		}
	}
	return result
}

func collectFlags(cmd *cobra.Command) []flagInfo {
	var out []flagInfo
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		out = append(out, flagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})
	return out
}
