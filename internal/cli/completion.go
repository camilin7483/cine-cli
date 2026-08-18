package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func (a *App) completionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for cine-cli.

To use:
  bash:   source <(cine completion bash)
          echo "source <(cine completion bash)" >> ~/.bashrc
  zsh:    source <(cine completion zsh)
          echo "source <(cine completion zsh)" >> ~/.zshrc
  fish:   cine completion fish | source
          cine completion fish > ~/.config/fish/completions/cine.fish
  powershell: .\cine completion powershell >> $PROFILE`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(args[0])
		},
	}
	return cmd
}

func runCompletion(shell string) error {
	root := &cobra.Command{Use: "cine"}
	switch shell {
	case "bash":
		return root.GenBashCompletion(os.Stdout)
	case "zsh":
		return root.GenZshCompletion(os.Stdout)
	case "fish":
		return root.GenFishCompletion(os.Stdout, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
	}
}
