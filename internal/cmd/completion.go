// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:     "completion [bash|zsh|fish|powershell]",
	GroupID: "utility",
	Short:   "Generate completion script for your shell",
	Long: `To load completions:

Bash:

  $ source <(erst completion bash)

  # To load completions for each session, add to your .bashrc:
  # (on macOS, you may need to install bash-completion)
  $ erst completion bash > /usr/local/etc/bash_completion.d/erst

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, add to your .zshrc:
  $ source <(erst completion zsh)

  # Alternatively, you can add the completion script to your fpath:
  $ erst completion zsh > "${fpath[1]}/_erst"

Fish:

  $ erst completion fish | source

  # To load completions for each session, add to your fish configuration file:
  $ erst completion fish > ~/.config/fish/completions/erst.fish

PowerShell:

  PS> erst completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> erst completion powershell > erst.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
