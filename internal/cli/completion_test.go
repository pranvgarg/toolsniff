package cli

import (
	"flag"
	"strings"
	"testing"
)

func TestRenderCompletionScriptListsEveryRegisteredFlag(t *testing.T) {
	fs := flag.NewFlagSet("toolsniff", flag.ContinueOnError)
	fs.Bool("list", false, "")
	fs.Bool("legacy-tabs", false, "")
	fs.Bool("intent-tabs", false, "")

	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := renderCompletionScript(shell, fs)
		if err != nil {
			t.Fatalf("renderCompletionScript(%s): %v", shell, err)
		}
		// Check for the flag names in the script. Bash and zsh include the "--"
		// prefix directly, but fish uses the flag name after "-l".
		for _, flagName := range []string{"list", "legacy-tabs", "intent-tabs"} {
			if !strings.Contains(script, flagName) {
				t.Errorf("%s completion missing %q", shell, flagName)
			}
		}
	}
}

func TestRenderCompletionScriptRejectsUnknownShell(t *testing.T) {
	fs := flag.NewFlagSet("toolsniff", flag.ContinueOnError)
	if _, err := renderCompletionScript("powershell", fs); err == nil {
		t.Fatal("expected an error for an unsupported shell")
	}
}
