package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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

func confirmUpdate(info update.UpdateInfo) (bool, error) {
	fmt.Printf("toolsniff %s is outdated via Homebrew (%s). Update now? [y/N] ", info.Name, info.Source)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(answer), "y"), nil
}

func main() {
	listFlag := flag.Bool("list", false, "print a plain grouped table and exit")
	jsonFlag := flag.Bool("json", false, "print the full scan as JSON and exit")
	saveFlag := flag.Bool("save", false, "scan, save the result as the new baseline, and exit")
	diffFlag := flag.Bool("diff", false, "scan, print only what changed since the last save, and exit")
	availableFlag := flag.Bool("available", false, "include PATH availability changes with --diff")
	updateFlag := flag.Bool("update", false, "update the Homebrew-installed toolsniff binary and exit")
	yesFlag := flag.Bool("yes", false, "confirm --update without prompting")
	versionFlag := flag.Bool("version", false, "print the toolsniff version and exit")
	configFlag := flag.String("config", config.DefaultConfigPath(), "path to the TOML configuration file")
	flag.Parse()
	appVersion := version.Current()

	if *versionFlag {
		fmt.Println(appVersion)
		return
	}

	selectedModes := 0
	for _, selected := range []*bool{listFlag, jsonFlag, saveFlag, diffFlag, updateFlag} {
		if *selected {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		fmt.Fprintln(os.Stderr, "only one of --list, --json, --save, --diff, or --update may be used")
		os.Exit(2)
	}
	if err := validateFlags(*availableFlag, *diffFlag, *updateFlag, *yesFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *updateFlag {
		result, err := update.NewService(nil).Run(update.Options{
			Yes:    *yesFlag,
			Prompt: confirmUpdate,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "toolsniff update failed: %v\n", err)
			var cltErr *update.CommandLineToolsError
			if errors.As(err, &cltErr) {
				fmt.Fprintln(os.Stderr, "Update Apple Command Line Tools through Software Update, then retry.")
			}
			os.Exit(1)
		}
		switch {
		case result.Updated:
			fmt.Printf("toolsniff updated through Homebrew (%s)\n", result.Status.Source)
		case result.Skipped:
			fmt.Println("toolsniff update cancelled")
		default:
			fmt.Printf("toolsniff is already up to date through Homebrew (%s)\n", result.Status.Source)
		}
		return
	}

	settings, err := config.Load(*configFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	registrations := buildScanners(settings)
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

	switch {
	case *saveFlag:
		if err := registry.Save(regPath, installedTools); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := registry.Save(availabilityPath, availableTools); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("saved baseline: %d installed tools, %d available commands\n", len(installedTools), len(availableTools))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", w.Source, w.Err)
		}

	case *diffFlag:
		fmt.Print(output.RenderDiff(diff))
		if *availableFlag {
			fmt.Println("AVAILABILITY CHANGES")
			fmt.Print(output.RenderDiff(availabilityDiff))
		}
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", w.Source, w.Err)
		}

	case *jsonFlag:
		data, err := output.RenderJSON(installedTools, availableTools, npxHistory, diff, registry.Diff{}, warnings)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	case *listFlag:
		fmt.Print(output.RenderTable(installedTools, availableTools, npxHistory, diff, registry.Diff{}, warnings))

	default:
		if err := output.RunTUI(installedTools, availableTools, npxHistory, diff, warnings, output.TUIOptions{
			Sources:      registrationSources(registrations),
			RegistryPath: regPath,
			Version:      appVersion,
			Theme:        settings.Theme,
			ConfigPath:   settings.ConfigPath,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
