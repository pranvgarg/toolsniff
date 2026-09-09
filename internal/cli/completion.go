package cli

import (
	"flag"
	"fmt"
	"strings"
)

// renderCompletionScript builds a shell completion script by walking the
// same *flag.FlagSet parseFlags constructs, so the script can never drift
// from the real flag set the way a hand-written completion list would.
func renderCompletionScript(shell string, fs *flag.FlagSet) (string, error) {
	var names []string
	fs.VisitAll(func(f *flag.Flag) {
		names = append(names, "--"+f.Name)
	})

	switch shell {
	case "bash":
		return renderBashCompletion(names), nil
	case "zsh":
		return renderZshCompletion(names), nil
	case "fish":
		return renderFishCompletion(names), nil
	default:
		return "", fmt.Errorf("completion: unsupported shell %q (want bash, zsh, or fish)", shell)
	}
}

func renderBashCompletion(flags []string) string {
	return fmt.Sprintf(`_toolsniff_completions() {
    COMPREPLY=($(compgen -W "%s" -- "${COMP_WORDS[COMP_CWORD]}"))
}
complete -F _toolsniff_completions toolsniff
`, strings.Join(flags, " "))
}

func renderZshCompletion(flags []string) string {
	var lines strings.Builder
	lines.WriteString("#compdef toolsniff\n_toolsniff() {\n  local -a opts\n  opts=(\n")
	for _, f := range flags {
		fmt.Fprintf(&lines, "    %q\n", f)
	}
	lines.WriteString("  )\n  _describe 'toolsniff flags' opts\n}\ncompdef _toolsniff toolsniff\n")
	return lines.String()
}

func renderFishCompletion(flags []string) string {
	var lines strings.Builder
	for _, f := range flags {
		name := strings.TrimPrefix(f, "--")
		fmt.Fprintf(&lines, "complete -c toolsniff -l %s\n", name)
	}
	return lines.String()
}
