package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCLISeparatesInstalledAndAvailabilityBaselines(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("creating fake PATH: %v", err)
	}
	configPath := filepath.Join(root, "config.toml")
	registryPath := filepath.Join(root, "registry.json")
	config := strings.Join([]string{
		"[applications]",
		"roots = [" + strconv.Quote(filepath.Join(root, "applications")) + "]",
		"",
		"[path]",
		"directories = [" + strconv.Quote(binDir) + "]",
		"",
		"[npx]",
		"dir = " + strconv.Quote(filepath.Join(root, "npx")),
		"",
		"[cargo]",
		"bin_dir = " + strconv.Quote(filepath.Join(root, "cargo")),
		"",
		"[bun]",
		"enabled = false",
		"",
		"[registry]",
		"path = " + strconv.Quote(registryPath),
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	if err := writeFakeExecutable(filepath.Join(binDir, "tool-a")); err != nil {
		t.Fatalf("writing first fake executable: %v", err)
	}

	binaryPath := filepath.Join(root, "toolsniff")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	buildOutput, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("building toolsniff: %v\n%s", err, buildOutput)
	}

	run := func(args ...string) string {
		cmd := exec.Command(binaryPath, args...)
		cmd.Env = isolatedTestEnvironment(binDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("toolsniff %s: %v\n%s", strings.Join(args, " "), err, output)
		}
		return string(output)
	}

	run("--config", configPath, "--save")
	installedData, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("reading installed registry: %v", err)
	}
	availabilityData, err := os.ReadFile(filepath.Join(root, "availability.json"))
	if err != nil {
		t.Fatalf("reading availability registry: %v", err)
	}
	if strings.Contains(string(installedData), "tool-a") {
		t.Fatalf("PATH tool leaked into installed registry: %s", installedData)
	}
	if !strings.Contains(string(availabilityData), "tool-a") {
		t.Fatalf("PATH tool missing from availability registry: %s", availabilityData)
	}

	if err := os.Remove(filepath.Join(binDir, "tool-a")); err != nil {
		t.Fatalf("removing first fake executable: %v", err)
	}
	if err := writeFakeExecutable(filepath.Join(binDir, "tool-b")); err != nil {
		t.Fatalf("writing second fake executable: %v", err)
	}

	diff := run("--config", configPath, "--diff", "--available")
	for _, want := range []string{
		"AVAILABILITY CHANGES",
		"+ tool-b (path)",
		"- tool-a (path)",
	} {
		if !strings.Contains(diff, want) {
			t.Errorf("availability diff missing %q:\n%s", want, diff)
		}
	}
}

func TestCLIUpdateUsesHomebrewFormula(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatalf("creating fake command directory: %v", err)
	}
	logPath := filepath.Join(root, "brew.log")
	fakeBrew := `#!/bin/sh
printf '%s\n' "$*" >> "$BREW_LOG"
case "$*" in
  "--version") printf 'Homebrew 4.6.0\n' ;;
  "list --formula toolsniff") printf 'toolsniff\n' ;;
  "list --cask toolsniff") exit 1 ;;
  "outdated --formula toolsniff") printf 'toolsniff 0.1.0 < 0.2.0\n' ;;
  "upgrade toolsniff") printf 'Upgrading toolsniff\n' ;;
  *) printf 'unexpected brew command: %s\n' "$*" >&2; exit 2 ;;
esac
`
	brewPath := filepath.Join(fakeBin, "brew")
	if err := os.WriteFile(brewPath, []byte(fakeBrew), 0o700); err != nil {
		t.Fatalf("writing fake brew: %v", err)
	}

	binaryPath := filepath.Join(root, "toolsniff")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	buildOutput, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("building toolsniff: %v\n%s", err, buildOutput)
	}

	cmd := exec.Command(binaryPath, "--update", "--yes")
	cmd.Env = append(isolatedTestEnvironment(fakeBin), "BREW_LOG="+logPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("toolsniff --update --yes: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "toolsniff updated through Homebrew (formula)") {
		t.Fatalf("unexpected update output: %s", output)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading fake brew log: %v", err)
	}
	if !strings.Contains(string(log), "upgrade toolsniff") {
		t.Fatalf("fake brew did not receive upgrade command: %s", log)
	}
}

func writeFakeExecutable(path string) error {
	return os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700)
}

func isolatedTestEnvironment(pathDir string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if value == "PATH=" || strings.HasPrefix(value, "PATH=") || strings.HasPrefix(value, "TOOLSNIFF_") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "PATH="+pathDir)
}
