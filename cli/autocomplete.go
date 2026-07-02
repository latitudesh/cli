package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func makeGenCompletionCmd() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate a shell completion script for lsh",
		Long: `Generate a shell completion script for lsh.

Run the line for your shell to enable tab-completion of commands and flags
for the current session, or follow the "persist" step to enable it for
every new shell.

Bash:
  # current shell (works with any install method)
  source <(lsh completion bash)
  # persist (any install): source it from ~/.bashrc
  lsh completion bash > ~/.lsh-completion.bash
  echo 'source ~/.lsh-completion.bash' >> ~/.bashrc
  # persist (system-wide; needs the bash-completion package, may need sudo):
  lsh completion bash > "$(brew --prefix)/etc/bash_completion.d/lsh"   # macOS, Homebrew
  lsh completion bash > /etc/bash_completion.d/lsh                     # Linux

Zsh (works with any install method):
  # enable completions once — add to ~/.zshrc, BEFORE any compinit call:
  #   fpath=(~/.zsh/completions $fpath)
  #   autoload -Uz compinit && compinit
  # then install the script there and restart your shell:
  mkdir -p ~/.zsh/completions
  lsh completion zsh > ~/.zsh/completions/_lsh
  # Homebrew shortcut (its site-functions dir is already on $fpath):
  lsh completion zsh > "$(brew --prefix)/share/zsh/site-functions/_lsh"

fish:
  # current shell
  lsh completion fish | source
  # persist
  lsh completion fish > ~/.config/fish/completions/lsh.fish

PowerShell:
  # current session
  lsh completion powershell | Out-String | Invoke-Expression
  # persist: add the line above to your PowerShell profile
`,
		Example: `  # load in the current shell (works everywhere)
  source <(lsh completion bash)

  # zsh, for every new shell (portable; see "Zsh" above for the one-time setup)
  mkdir -p ~/.zsh/completions && lsh completion zsh > ~/.zsh/completions/_lsh`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
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
	return completionCmd
}
