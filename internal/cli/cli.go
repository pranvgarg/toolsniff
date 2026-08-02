package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/config"
	"github.com/pranvgarg/toolsniff/internal/update"
	"github.com/pranvgarg/toolsniff/internal/version"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// Run executes the toolsniff command and returns the process exit status.
// Keeping process I/O at the boundary makes command-line behavior testable
// without replacing os.Stdin, os.Stdout, or os.Stderr.
func Run(args []string, input io.Reader, outputWriter io.Writer, errorOutput io.Writer) int {
	if input == nil {
		input = strings.NewReader("")
	}
	if outputWriter == nil {
		outputWriter = io.Discard
	}
	if errorOutput == nil {
		errorOutput = io.Discard
	}

	options, err := parseFlags(args, errorOutput)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	appVersion := version.Current()

	if options.version {
		fmt.Fprintln(outputWriter, appVersion)
		return 0
	}
	if err := validateMode(options); err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}
	if options.update {
		return runUpdate(options.yes, input, outputWriter, errorOutput)
	}

	settings, err := config.Load(options.configPath)
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}

	registrations := buildScanners(settings)
	tools, warnings := scanTools(registrations)
	installedTools, availableTools, npxHistory := splitByRole(tools, registrations)

	regPath := settings.RegistryPath
	baseline, regWarning := registry.Load(regPath)
	if regWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "registry", Err: errors.New(regWarning)})
	}
	availabilityPath := registry.AvailabilityPath(regPath)
	availabilityBaseline, availabilityWarning := registry.Load(availabilityPath)
	if availabilityWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "availability-registry", Err: errors.New(availabilityWarning)})
	}
	diff := registry.ComputeDiff(baseline, installedTools)
	availabilityDiff := registry.ComputeDiff(availabilityBaseline, availableTools)

	return dispatchReport(options, settings, registrations, installedTools, availableTools, npxHistory, diff, availabilityDiff, warnings, appVersion, outputWriter, errorOutput)
}

type cliOptions struct {
	list       bool
	json       bool
	save       bool
	diff       bool
	available  bool
	update     bool
	yes        bool
	version    bool
	configPath string
}

func parseFlags(args []string, errorOutput io.Writer) (cliOptions, error) {
	flags := flag.NewFlagSet("toolsniff", flag.ContinueOnError)
	flags.SetOutput(errorOutput)

	var options cliOptions
	flags.BoolVar(&options.list, "list", false, "print a plain grouped table and exit")
	flags.BoolVar(&options.json, "json", false, "print the full scan as JSON and exit")
	flags.BoolVar(&options.save, "save", false, "scan, save the result as the new baseline, and exit")
	flags.BoolVar(&options.diff, "diff", false, "scan, print only what changed since the last save, and exit")
	flags.BoolVar(&options.available, "available", false, "include PATH availability changes with --diff")
	flags.BoolVar(&options.update, "update", false, "update the Homebrew-installed toolsniff binary and exit")
	flags.BoolVar(&options.yes, "yes", false, "confirm --update without prompting")
	flags.BoolVar(&options.version, "version", false, "print the toolsniff version and exit")
	flags.StringVar(&options.configPath, "config", config.DefaultConfigPath(), "path to the TOML configuration file")

	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	return options, nil
}

func validateMode(options cliOptions) error {
	selectedModes := 0
	for _, selected := range []bool{options.list, options.json, options.save, options.diff, options.update} {
		if selected {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		return errors.New("only one of --list, --json, --save, --diff, or --update may be used")
	}
	return validateFlags(options.available, options.diff, options.update, options.yes)
}

func buildScanners(settings config.Settings) []scanner.Registration {
	runner := scanner.NewExecRunner(settings.ExecTimeout)
	registrations := []scanner.Registration{
		{SourceInfo: scanner.SourceInfo{ID: model.SourceNPM, Order: 10, Role: model.RoleInstalled}, Scanner: scanner.NewNPMScanner(runner)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceNPXHistory, Order: 90, Role: model.RoleHistory, Informational: true}, Scanner: scanner.NewNPXScanner(settings.NPXDir)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceBrewFormula, Order: 20, Role: model.RoleInstalled}, Scanner: scanner.NewHomebrewFormulaScanner(runner)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceBrewCask, Order: 30, Role: model.RoleInstalled}, Scanner: scanner.NewHomebrewCaskScanner(runner)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourcePipx, Order: 40, Role: model.RoleInstalled}, Scanner: scanner.NewPipxScanner(runner)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceCargo, Order: 50, Role: model.RoleInstalled}, Scanner: scanner.NewCargoScanner(settings.CargoBinDir)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceApplications, Order: 60, Role: model.RoleInstalled}, Scanner: scanner.NewApplicationsScanner(settings.Applications.Roots, settings.Applications.IgnorePath)},
		{SourceInfo: scanner.SourceInfo{ID: model.SourcePath, Order: 80, Role: model.RoleAvailable}, Scanner: scanner.NewPathScanner(settings.Path.Directories, settings.Path.Excluded, settings.Path.IgnoreNames)},
	}
	if settings.Bun.Enabled {
		registrations = append(registrations, scanner.Registration{
			SourceInfo: scanner.SourceInfo{ID: model.SourceBun, Order: 70, Role: model.RoleInstalled},
			Scanner:    scanner.NewBunScanner(runner),
		})
	}
	sort.Slice(registrations, func(i, j int) bool {
		return registrations[i].Order < registrations[j].Order
	})
	return registrations
}

func scanTools(registrations []scanner.Registration) ([]model.Tool, []scanner.Warning) {
	scanners := make([]scanner.Scanner, 0, len(registrations))
	for _, registration := range registrations {
		scanners = append(scanners, registration.Scanner)
	}
	tools, warnings := scanner.RunAll(scanners)
	tools = model.DeduplicateTools(tools)
	annotateTools(tools, registrations)
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Source != tools[j].Source {
			return tools[i].Source < tools[j].Source
		}
		if tools[i].Name != tools[j].Name {
			return tools[i].Name < tools[j].Name
		}
		return tools[i].Path < tools[j].Path
	})
	return tools, warnings
}

func annotateTools(tools []model.Tool, registrations []scanner.Registration) {
	roles := make(map[string]model.SourceRole, len(registrations))
	for _, registration := range registrations {
		roles[registration.ID] = registration.Role
	}
	for i := range tools {
		if role, ok := roles[tools[i].Source]; ok {
			tools[i].Role = role
		}
	}
}

func splitByRole(tools []model.Tool, registrations []scanner.Registration) (installed, available, history []model.Tool) {
	roles := make(map[string]scanner.SourceInfo, len(registrations))
	for _, registration := range registrations {
		roles[registration.ID] = registration.SourceInfo
	}
	for _, tool := range tools {
		info := roles[tool.Source]
		switch {
		case info.Informational || info.Role == model.RoleHistory:
			history = append(history, tool)
		case info.Role == model.RoleAvailable:
			available = append(available, tool)
		default:
			installed = append(installed, tool)
		}
	}
	return installed, available, history
}

func registrationSources(registrations []scanner.Registration) []scanner.SourceInfo {
	sources := make([]scanner.SourceInfo, 0, len(registrations))
	for _, registration := range registrations {
		sources = append(sources, registration.SourceInfo)
	}
	return sources
}

func validateFlags(available, diff, updateMode, yes bool) error {
	if available && !diff {
		return errors.New("--available may only be used with --diff")
	}
	if yes && !updateMode {
		return errors.New("--yes may only be used with --update")
	}
	return nil
}

func dispatchReport(options cliOptions, settings config.Settings, registrations []scanner.Registration, installedTools, availableTools, npxHistory []model.Tool, diff, availabilityDiff registry.Diff, warnings []scanner.Warning, appVersion string, outputWriter, errorOutput io.Writer) int {
	regPath := settings.RegistryPath
	switch {
	case options.save:
		if err := registry.Save(regPath, installedTools); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		if err := registry.Save(registry.AvailabilityPath(regPath), availableTools); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "saved baseline: %d installed tools, %d available commands\n", len(installedTools), len(availableTools))
		writeWarnings(errorOutput, warnings)
	case options.diff:
		fmt.Fprint(outputWriter, output.RenderDiff(diff))
		if options.available {
			fmt.Fprintln(outputWriter, "AVAILABILITY CHANGES")
			fmt.Fprint(outputWriter, output.RenderDiff(availabilityDiff))
		}
		writeWarnings(errorOutput, warnings)
	case options.json:
		data, err := output.RenderJSON(installedTools, availableTools, npxHistory, diff, registry.Diff{}, warnings)
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintln(outputWriter, string(data))
	case options.list:
		fmt.Fprint(outputWriter, output.RenderTable(installedTools, availableTools, npxHistory, diff, registry.Diff{}, warnings))
	default:
		if err := output.RunTUI(installedTools, availableTools, npxHistory, diff, warnings, output.TUIOptions{
			Sources:      registrationSources(registrations),
			RegistryPath: regPath,
			Version:      appVersion,
			Theme:        settings.Theme,
			ConfigPath:   settings.ConfigPath,
		}); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
	}
	return 0
}

func writeWarnings(errorOutput io.Writer, warnings []scanner.Warning) {
	for _, warning := range warnings {
		fmt.Fprintf(errorOutput, "warning: %s: %v\n", warning.Source, warning.Err)
	}
}

func runUpdate(yes bool, input io.Reader, outputWriter, errorOutput io.Writer) int {
	result, err := update.NewService(nil).Run(update.Options{
		Yes: yes,
		Prompt: func(info update.UpdateInfo) (bool, error) {
			return confirmUpdate(info, input, outputWriter)
		},
	})
	if err != nil {
		fmt.Fprintf(errorOutput, "toolsniff update failed: %v\n", err)
		var cltErr *update.CommandLineToolsError
		if errors.As(err, &cltErr) {
			fmt.Fprintln(errorOutput, "Update Apple Command Line Tools through Software Update, then retry.")
		}
		return 1
	}
	switch {
	case result.Updated:
		fmt.Fprintf(outputWriter, "toolsniff updated through Homebrew (%s)\n", result.Status.Source)
	case result.Skipped:
		fmt.Fprintln(outputWriter, "toolsniff update cancelled")
	default:
		fmt.Fprintf(outputWriter, "toolsniff is already up to date through Homebrew (%s)\n", result.Status.Source)
	}
	return 0
}

func confirmUpdate(info update.UpdateInfo, input io.Reader, outputWriter io.Writer) (bool, error) {
	fmt.Fprintf(outputWriter, "toolsniff %s is outdated via Homebrew (%s). Update now? [y/N] ", info.Name, info.Source)
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(answer), "y"), nil
}
