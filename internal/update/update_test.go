package update

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeCommand struct {
	name string
	args []string
	out  []byte
	err  error
}

type fakeRunner struct {
	responses map[string]fakeCommand
	calls     []fakeCommand
}

func (f *fakeRunner) run(name string, args ...string) ([]byte, error) {
	call := fakeCommand{name: name, args: append([]string(nil), args...)}
	f.calls = append(f.calls, call)
	response, ok := f.responses[commandKey(name, args...)]
	if !ok {
		return nil, errors.New("unexpected command: " + commandKey(name, args...))
	}
	return response.out, response.err
}

func TestDetectHomebrew(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		fake := &fakeRunner{responses: map[string]fakeCommand{
			"brew --version": {out: []byte("Homebrew 4.6.0\n")},
		}}
		if err := NewService(fake.run).DetectHomebrew(); err != nil {
			t.Fatalf("DetectHomebrew() error = %v", err)
		}
		assertCalls(t, fake.calls, "brew --version")
	})

	t.Run("missing", func(t *testing.T) {
		fake := &fakeRunner{responses: map[string]fakeCommand{
			"brew --version": {err: errors.New("exec: \\\"brew\\\": executable file not found in $PATH")},
		}}
		err := NewService(fake.run).DetectHomebrew()
		if !errors.Is(err, ErrHomebrewNotInstalled) {
			t.Fatalf("error = %v, want ErrHomebrewNotInstalled", err)
		}
	})
}

func TestDetectInstallation(t *testing.T) {
	tests := []struct {
		name       string
		formulaOut []byte
		formulaErr error
		caskOut    []byte
		caskErr    error
		want       Source
		wantErr    error
	}{
		{name: "formula", formulaOut: []byte("toolsniff\n"), caskErr: errors.New("Error: Cask toolsniff is not installed"), want: SourceFormula},
		{name: "cask", formulaErr: errors.New("Error: Formula toolsniff is not installed"), caskOut: []byte("toolsniff\n"), want: SourceCask},
		{name: "neither", formulaErr: errors.New("not installed"), caskErr: errors.New("not installed"), wantErr: ErrToolsniffNotInstalled},
		{name: "ambiguous", formulaOut: []byte("toolsniff\n"), caskOut: []byte("toolsniff\n"), wantErr: ErrAmbiguousInstallation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRunner{responses: map[string]fakeCommand{
				"brew --version":                {out: []byte("Homebrew 4.6.0\n")},
				"brew list --formula toolsniff": {out: tt.formulaOut, err: tt.formulaErr},
				"brew list --cask toolsniff":    {out: tt.caskOut, err: tt.caskErr},
			}}
			got, err := NewService(fake.run).DetectInstallation()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("source = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckOutdatedIsSourceSpecific(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		out      []byte
		want     bool
		wantCall string
	}{
		{name: "formula outdated", source: SourceFormula, out: []byte("toolsniff 0.1.0 < 0.2.0\n"), want: true, wantCall: "brew outdated --formula toolsniff"},
		{name: "cask current", source: SourceCask, wantCall: "brew outdated --cask toolsniff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRunner{responses: map[string]fakeCommand{
				tt.wantCall: {out: tt.out},
			}}
			got, err := NewService(fake.run).IsOutdated(tt.source)
			if err != nil {
				t.Fatalf("IsOutdated() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("outdated = %v, want %v", got, tt.want)
			}
			assertCalls(t, fake.calls, tt.wantCall)
		})
	}
}

func TestRunPromptsAndUpgradesFormula(t *testing.T) {
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version":                    {out: []byte("Homebrew 4.6.0\n")},
		"brew list --formula toolsniff":     {out: []byte("toolsniff\n")},
		"brew list --cask toolsniff":        {err: errors.New("not installed")},
		"brew outdated --formula toolsniff": {out: []byte("toolsniff 0.1.0 < 0.2.0\n")},
		"brew upgrade toolsniff":            {out: []byte("Upgrading toolsniff\n")},
	}}
	var gotInfo UpdateInfo
	result, err := NewService(fake.run).Run(Options{
		Prompt: func(info UpdateInfo) (bool, error) {
			gotInfo = info
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Updated || result.Skipped || !result.Status.Outdated {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotInfo.Name != ToolName || gotInfo.Source != SourceFormula || gotInfo.OutdatedOutput == "" {
		t.Errorf("unexpected prompt info: %+v", gotInfo)
	}
	assertCalls(t, fake.calls,
		"brew --version",
		"brew list --formula toolsniff",
		"brew list --cask toolsniff",
		"brew outdated --formula toolsniff",
		"brew upgrade toolsniff",
	)
}

func TestRunYesSkipsPromptAndUpgradesCask(t *testing.T) {
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version":                 {out: []byte("Homebrew 4.6.0\n")},
		"brew list --formula toolsniff":  {err: errors.New("not installed")},
		"brew list --cask toolsniff":     {out: []byte("toolsniff\n")},
		"brew outdated --cask toolsniff": {out: []byte("toolsniff 0.1.0 < 0.2.0\n")},
		"brew upgrade --cask toolsniff":  {out: []byte("Upgrading toolsniff\n")},
	}}
	promptCalled := false
	result, err := NewService(fake.run).Run(Options{
		Yes: true,
		Prompt: func(UpdateInfo) (bool, error) {
			promptCalled = true
			return false, nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Updated || promptCalled {
		t.Errorf("result = %+v, promptCalled = %v", result, promptCalled)
	}
	assertCalls(t, fake.calls, "brew --version", "brew list --formula toolsniff", "brew list --cask toolsniff", "brew outdated --cask toolsniff", "brew upgrade --cask toolsniff")
}

func TestRunDeclinedDoesNotUpgrade(t *testing.T) {
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version":                    {out: []byte("Homebrew 4.6.0\n")},
		"brew list --formula toolsniff":     {out: []byte("toolsniff\n")},
		"brew list --cask toolsniff":        {err: errors.New("not installed")},
		"brew outdated --formula toolsniff": {out: []byte("toolsniff 0.1.0 < 0.2.0\n")},
	}}
	result, err := NewService(fake.run).Run(Options{Prompt: func(UpdateInfo) (bool, error) { return false, nil }})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Updated || !result.Skipped {
		t.Errorf("unexpected result: %+v", result)
	}
	assertCalls(t, fake.calls, "brew --version", "brew list --formula toolsniff", "brew list --cask toolsniff", "brew outdated --formula toolsniff")
}

func TestCommandLineToolsErrorsAreActionable(t *testing.T) {
	message := errors.New("Error: Your Command Line Tools are too outdated.")
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version": {err: message},
	}}
	err := NewService(fake.run).DetectHomebrew()
	var cltErr *CommandLineToolsError
	if !errors.As(err, &cltErr) || !errors.Is(err, ErrCommandLineToolsOutdated) {
		t.Fatalf("error = %v, want actionable CommandLineToolsError", err)
	}
	if !strings.Contains(err.Error(), "Software Update") {
		t.Errorf("error = %q, want Software Update guidance", err)
	}

	fake = &fakeRunner{responses: map[string]fakeCommand{
		"brew outdated --formula toolsniff": {out: []byte("Your Command Line Tools are too outdated"), err: errors.New("exit status 1")},
	}}
	_, err = NewService(fake.run).IsOutdated(SourceFormula)
	if !errors.Is(err, ErrCommandLineToolsOutdated) {
		t.Fatalf("outdated error = %v, want ErrCommandLineToolsOutdated", err)
	}
}

func TestRunPromptErrorIsReturned(t *testing.T) {
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version":                    {out: []byte("Homebrew 4.6.0\n")},
		"brew list --formula toolsniff":     {out: []byte("toolsniff\n")},
		"brew list --cask toolsniff":        {err: errors.New("not installed")},
		"brew outdated --formula toolsniff": {out: []byte("toolsniff 0.1.0 < 0.2.0\n")},
	}}
	want := errors.New("input closed")
	_, err := NewService(fake.run).Run(Options{Prompt: func(UpdateInfo) (bool, error) { return false, want }})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRunCurrentDoesNotPromptOrUpgrade(t *testing.T) {
	fake := &fakeRunner{responses: map[string]fakeCommand{
		"brew --version":                    {out: []byte("Homebrew 4.6.0\n")},
		"brew list --formula toolsniff":     {out: []byte("toolsniff\n")},
		"brew list --cask toolsniff":        {err: errors.New("not installed")},
		"brew outdated --formula toolsniff": {out: nil},
	}}
	promptCalled := false
	result, err := NewService(fake.run).Run(Options{Prompt: func(UpdateInfo) (bool, error) {
		promptCalled = true
		return true, nil
	}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Updated || result.Skipped || promptCalled {
		t.Errorf("unexpected result: %+v, promptCalled = %v", result, promptCalled)
	}
}

func commandKey(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func assertCalls(t *testing.T, calls []fakeCommand, want ...string) {
	t.Helper()
	got := make([]string, 0, len(calls))
	for _, call := range calls {
		got = append(got, commandKey(call.name, call.args...))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}
