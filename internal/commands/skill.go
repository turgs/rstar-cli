package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/skills"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install or print the agent skill",
}

var skillInstallDir string
var skillPrint bool

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the rstar skill into an agents skills directory",
	Long: "Writes skills/rstar/SKILL.md for coding agents that load shared skills.\n\n" +
		"Default destination: $HOME/.agents/skills/rstar (override with --dir).",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if skillPrint {
			fmt.Fprint(stdout, skills.Markdown)
			return nil
		}
		dir := skillInstallDir
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return output.ErrConfig(err.Error())
			}
			dir = filepath.Join(home, ".agents", "skills", "rstar")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return output.ErrConfig(err.Error())
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(skills.Markdown), 0o644); err != nil {
			return output.ErrConfig(err.Error())
		}
		data := map[string]any{"path": path}
		if flagJSON {
			return output.WriteSuccess(stdout, data, "skill installed")
		}
		fmt.Fprintf(stdout, "Installed skill at %s\n", path)
		return nil
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the embedded agent skill markdown",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(stdout, skills.Markdown)
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().StringVar(&skillInstallDir, "dir", "", "Destination directory for SKILL.md")
	skillInstallCmd.Flags().BoolVar(&skillPrint, "print", false, "Print skill markdown instead of installing")
	skillCmd.AddCommand(skillInstallCmd, skillShowCmd)
}
