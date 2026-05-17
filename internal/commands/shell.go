package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/ui"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Shell integration utilities",
	Long:  "Install and manage sshelf's shell integration so you can switch profiles without eval $().",
	Args:  cobra.ArbitraryArgs,
	RunE:  groupRunE,
}

var shellSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install the sshelf-switch function into your shell config",
	Long: `Appends a sshelf-switch() function to your shell's config file (~/.zshrc,
~/.bashrc, or ~/.config/fish/config.fish).

After running this command once and opening a new terminal (or sourcing your
config), switch profiles with a single command:

  sshelf-switch work
  sshelf-switch personal

instead of:

  eval $(sshelf profile switch work)`,
	RunE: func(_ *cobra.Command, _ []string) error {
		shell := detectShell()
		rcFile, err := shellRCFile(shell)
		if err != nil {
			return err
		}

		snippet := shellSnippet(shell)
		const marker = "# sshelf shell integration"

		existing, _ := os.ReadFile(rcFile)
		if strings.Contains(string(existing), marker) {
			fmt.Printf("%s Shell integration already installed in %s\n",
				ui.StyleGreen.Render("✓"), rcFile)
			fmt.Println()
			fmt.Println("Switch profiles with:")
			fmt.Println("  sshelf-switch work")
			fmt.Println("  sshelf-switch personal")
			return nil
		}

		// Ensure parent directory exists (relevant for fish config).
		if err := os.MkdirAll(filepath.Dir(rcFile), 0700); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}

		f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open %s: %w", rcFile, err)
		}
		defer f.Close()

		if _, err := fmt.Fprintf(f, "\n%s\n", snippet); err != nil {
			return fmt.Errorf("write to %s: %w", rcFile, err)
		}

		fmt.Printf("%s Shell integration installed in %s\n\n",
			ui.StyleGreen.Render("✓"), rcFile)
		fmt.Printf("Apply now (or open a new terminal):\n  source %s\n\n", rcFile)
		fmt.Println("Then switch profiles with:")
		fmt.Println("  sshelf-switch work")
		fmt.Println("  sshelf-switch personal")
		return nil
	},
}

func init() {
	shellCmd.AddCommand(shellSetupCmd)
}

// stdoutIsTTY returns true when stdout is connected directly to a terminal
// (i.e. the user ran the command without piping or eval).
func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func detectShell() string {
	base := filepath.Base(os.Getenv("SHELL"))
	switch base {
	case "zsh", "bash", "fish":
		return base
	default:
		return "bash"
	}
}

func shellRCFile(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		return filepath.Join(home, ".bashrc"), nil
	}
}

func shellSnippet(shell string) string {
	switch shell {
	case "fish":
		return `# sshelf shell integration
function sshelf-switch
  eval (sshelf profile switch $argv)
end`
	default: // zsh / bash
		return `# sshelf shell integration
sshelf-switch() {
  eval $(sshelf profile switch "$1")
}`
	}
}
