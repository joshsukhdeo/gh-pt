package cmd

import (
	"fmt"
)

func GenerateCompletions(shell string) error {
	binName := "gh-pt"
	aliasName := "ghpt"

	switch shell {
	case "bash":
		fmt.Printf("complete -C %s %s\n", binName, binName)
		fmt.Printf("complete -C %s %s\n", binName, aliasName)
	case "zsh":
		fmt.Printf("autoload -U +X bashcompinit && bashcompinit\n")
		fmt.Printf("complete -C %s %s\n", binName, binName)
		fmt.Printf("complete -C %s %s\n", binName, aliasName)
	case "powershell":
		fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $env:COMP_LINE = $commandAst.ToString()
    $env:COMP_POINT = $cursorPosition
    %s | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
    Remove-Item Env:\COMP_LINE
    Remove-Item Env:\COMP_POINT
}
`, binName, binName)
		fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $env:COMP_LINE = $commandAst.ToString()
    $env:COMP_POINT = $cursorPosition
    %s | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
    Remove-Item Env:\COMP_LINE
    Remove-Item Env:\COMP_POINT
}
`, aliasName, binName)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
}
