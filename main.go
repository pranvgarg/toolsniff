package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/pranvgarg/toolsniff/config"
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

func splitByRole(tools []model.Tool, registrations []scanner.Registration) (real, history []model.Tool) {
	historySources := make(map[string]struct{})
	for _, registration := range registrations {
		if registration.Informational || registration.Role == model.RoleHistory {
			historySources[registration.ID] = struct{}{}
		}
	}
	for _, tool := range tools {
		if _, ok := historySources[tool.Source]; ok {
			history = append(history, tool)
		} else {
			real = append(real, tool)
		}
	}
	return real, history
}

func registrationSources(registrations []scanner.Registration) []scanner.SourceInfo {
	sources := make([]scanner.SourceInfo, 0, len(registrations))
	for _, registration := range registrations {
		sources = append(sources, registration.SourceInfo)
	}
	return sources
}

func main() {
	listFlag := flag.Bool("list", false, "print a plain grouped table and exit")
	jsonFlag := flag.Bool("json", false, "print the full scan as JSON and exit")
	saveFlag := flag.Bool("save", false, "scan, save the result as the new baseline, and exit")
	diffFlag := flag.Bool("diff", false, "scan, print only what changed since the last save, and exit")
	versionFlag := flag.Bool("version", false, "print the toolsniff version and exit")
	configFlag := flag.String("config", config.DefaultConfigPath(), "path to the TOML configuration file")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version.Version)
		return
	}

	selectedModes := 0
	for _, selected := range []*bool{listFlag, jsonFlag, saveFlag, diffFlag} {
		if *selected {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		fmt.Fprintln(os.Stderr, "only one of --list, --json, --save, or --diff may be used")
		os.Exit(2)
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
	realTools, npxHistory := splitByRole(tools, registrations)

	regPath := settings.RegistryPath
	baseline, regWarning := registry.Load(regPath)
	if regWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "registry", Err: errors.New(regWarning)})
	}
	diff := registry.ComputeDiff(baseline, realTools)

	switch {
	case *saveFlag:
		if err := registry.Save(regPath, realTools); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("saved baseline: %d tools\n", len(realTools))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", w.Source, w.Err)
		}

	case *diffFlag:
		fmt.Print(output.RenderDiff(diff))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", w.Source, w.Err)
		}

	case *jsonFlag:
		data, err := output.RenderJSON(realTools, npxHistory, diff, warnings)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	case *listFlag:
		fmt.Print(output.RenderTable(realTools, npxHistory, diff, warnings))

	default:
		if err := output.RunTUI(realTools, npxHistory, diff, warnings, output.TUIOptions{
			Sources:      registrationSources(registrations),
			RegistryPath: regPath,
			Version:      version.Version,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
