// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script for your shell",
	Long: `Generate shell completion scripts for erst commands.

The completion script must be evaluated to provide interactive completion of erst commands.
This can be done by sourcing it from your shell profile or piping it to the appropriate location.

Installation instructions:

  Bash:
    $ erst completion bash > /etc/bash_completion.d/erst
    $ source ~/.bashrc

  Zsh:
    $ erst completion zsh > "${fpath[1]}/_erst"
    # or place in your custom completions directory:
    $ mkdir -p ~/.zsh/completions
    $ erst completion zsh > ~/.zsh/completions/_erst
    # then add to your ~/.zshrc: fpath=(~/.zsh/completions $fpath)

  Fish:
    $ erst completion fish > ~/.config/fish/completions/erst.fish
    $ source ~/.config/fish/config.fish

  PowerShell:
    PS> erst completion powershell | Out-String | Invoke-Expression
    # To load completions for every new session, add to your PowerShell profile:
    PS> erst completion powershell >> $PROFILE

For detailed instructions on setting up completions for your shell, consult your shell's documentation.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]

		switch shell {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell type %q. Valid shells: bash, zsh, fish, powershell", shell)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
