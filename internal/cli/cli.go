package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pranvgarg/toolsniff/capabilities"
	"github.com/pranvgarg/toolsniff/config"
	"github.com/pranvgarg/toolsniff/diagnostics"
	"github.com/pranvgarg/toolsniff/internal/update"
	"github.com/pranvgarg/toolsniff/internal/version"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/profile"
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

	if err := validateMode(options); err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}
	if options.version {
		fmt.Fprintln(outputWriter, appVersion)
		return 0
	}
	if options.update {
		return runUpdate(options.yes, input, outputWriter, errorOutput)
	}
	if options.snapshots {
		return listSnapshots(outputWriter, errorOutput)
	}
	if options.initConfig {
		if err := config.WriteDefaultConfig(options.configPath, options.yes); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "wrote starter config: %s\n", options.configPath)
		return 0
	}

	settings, err := config.Load(options.configPath)
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}

	registrations := buildScanners(settings)
	observations, warnings := scanner.RunObservations(registrations)
	installedObservations, availableObservations, historyObservations := splitObservations(observations)

	regPath := settings.RegistryPath
	_, baselineStatErr := os.Stat(regPath)
	baselineExists := baselineStatErr == nil
	baseline, regWarning := registry.LoadObservations(regPath)
	if regWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "registry", Err: errors.New(regWarning)})
	}
	availabilityPath := registry.AvailabilityPath(regPath)
	availabilityBaseline, availabilityWarning := registry.LoadObservations(availabilityPath)
	if availabilityWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "availability-registry", Err: errors.New(availabilityWarning)})
	}
	diff := registry.ComputeObservationDiff(baseline, installedObservations)
	availabilityDiff := registry.ComputeObservationDiff(availabilityBaseline, availableObservations)
	reportChanges := mergeObservationDiffs(diff, availabilityDiff)
	report := output.NewObservationReport(installedObservations, availableObservations, historyObservations, reportChanges, warningStrings(warnings))

	return dispatchReport(options, settings, registrations, installedObservations, availableObservations, report, diff, availabilityDiff, warnings, appVersion, baselineExists, outputWriter, errorOutput)
}

type cliOptions struct {
	list              bool
	json              bool
	save              bool
	diff              bool
	available         bool
	doctor            bool
	capabilities      bool
	capabilitiesProbe bool
	snapshot          bool
	snapshots         bool
	export            string
	exportSet         bool
	compare           string
	compareSet        bool
	support           string
	supportSet        bool
	update            bool
	yes               bool
	version           bool
	initConfig        bool
	configPath        string
	legacyTabs        bool
	intentTabs        bool
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
	flags.BoolVar(&options.doctor, "doctor", false, "scan and print a read-only health and provenance report")
	flags.BoolVar(&options.capabilities, "capabilities", false, "scan and print explicit capability results as JSON")
	flags.BoolVar(&options.capabilitiesProbe, "capabilities-probe", false, "opt in to bounded version probes for --capabilities")
	flags.BoolVar(&options.snapshot, "snapshot", false, "scan and save non-history observations as a snapshot")
	flags.BoolVar(&options.snapshots, "snapshots", false, "list saved snapshots without scanning")
	flags.StringVar(&options.export, "export-profile", "", "scan and export a sanitized profile to FILE")
	flags.StringVar(&options.compare, "compare-profile", "", "scan and compare observations with profile FILE")
	flags.StringVar(&options.support, "support-bundle", "", "scan and write a sanitized support bundle to FILE")
	flags.BoolVar(&options.update, "update", false, "update the Homebrew-installed toolsniff binary and exit")
	flags.BoolVar(&options.yes, "yes", false, "confirm --update without prompting")
	flags.BoolVar(&options.version, "version", false, "print the toolsniff version and exit")
	flags.BoolVar(&options.initConfig, "init-config", false, "write a starter TOML config to --config's path and exit")
	flags.StringVar(&options.configPath, "config", config.DefaultConfigPath(), "path to the TOML configuration file")
	flags.BoolVar(&options.legacyTabs, "legacy-tabs", false, "use the original eight-tab layout instead of the intent-based tabs")
	flags.BoolVar(&options.intentTabs, "intent-tabs", false, "use the intent-based four-tab layout (Manage/Discover/Review/Health) instead of the eight-tab layout")

	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	flags.Visit(func(flag *flag.Flag) {
		switch flag.Name {
		case "export-profile":
			options.exportSet = true
		case "compare-profile":
			options.compareSet = true
		case "support-bundle":
			options.supportSet = true
		}
	})
	return options, nil
}

func validateMode(options cliOptions) error {
	selectedModes := 0
	for _, selected := range []bool{
		options.list,
		options.json,
		options.save,
		options.diff,
		options.doctor,
		options.capabilities,
		options.snapshot,
		options.snapshots,
		options.update,
		options.version,
		options.initConfig,
		options.exportSet,
		options.compareSet,
		options.supportSet,
	} {
		if selected {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		if options.list || options.json || options.save || options.diff || options.update {
			return errors.New("only one of --list, --json, --save, --diff, or --update may be used")
		}
		return errors.New("only one report, snapshot, profile, or update mode may be used")
	}
	if options.exportSet && options.export == "" {
		return errors.New("--export-profile requires FILE")
	}
	if options.compareSet && options.compare == "" {
		return errors.New("--compare-profile requires FILE")
	}
	if options.supportSet && options.support == "" {
		return errors.New("--support-bundle requires FILE")
	}
	if options.capabilitiesProbe && !options.capabilities {
		return errors.New("--capabilities-probe may only be used with --capabilities")
	}
	return validateFlags(options.available, options.diff, options.update, options.initConfig, options.yes)
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

func splitObservations(observations []model.Observation) (installed, available, history []model.Observation) {
	for _, observation := range observations {
		switch observation.Role {
		case model.RoleHistory:
			history = append(history, observation)
		case model.RoleAvailable:
			available = append(available, observation)
		default:
			installed = append(installed, observation)
		}
	}
	return installed, available, history
}

func mergeObservationDiffs(left, right registry.ObservationDiff) registry.ObservationDiff {
	return registry.ObservationDiff{
		Added:     append(append([]registry.ChangeEvent{}, left.Added...), right.Added...),
		Removed:   append(append([]registry.ChangeEvent{}, left.Removed...), right.Removed...),
		Updated:   append(append([]registry.ChangeEvent{}, left.Updated...), right.Updated...),
		Relocated: append(append([]registry.ChangeEvent{}, left.Relocated...), right.Relocated...),
		Broken:    append(append([]registry.ChangeEvent{}, left.Broken...), right.Broken...),
		Repaired:  append(append([]registry.ChangeEvent{}, left.Repaired...), right.Repaired...),
		Shadowed:  append(append([]registry.ChangeEvent{}, left.Shadowed...), right.Shadowed...),
	}
}

func warningStrings(warnings []scanner.Warning) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, fmt.Sprintf("%s: %v", warning.Source, warning.Err))
	}
	return result
}

func registrationSources(registrations []scanner.Registration) []scanner.SourceInfo {
	sources := make([]scanner.SourceInfo, 0, len(registrations))
	for _, registration := range registrations {
		sources = append(sources, registration.SourceInfo)
	}
	return sources
}

func validateFlags(available, diff, updateMode, initConfigMode, yes bool) error {
	if available && !diff {
		return errors.New("--available may only be used with --diff")
	}
	if yes && !updateMode && !initConfigMode {
		return errors.New("--yes may only be used with --update or --init-config")
	}
	return nil
}

func dispatchReport(options cliOptions, settings config.Settings, registrations []scanner.Registration, installedObservations, availableObservations []model.Observation, report output.ObservationReport, diff, availabilityDiff registry.ObservationDiff, warnings []scanner.Warning, appVersion string, baselineExists bool, outputWriter, errorOutput io.Writer) int {
	regPath := settings.RegistryPath
	switch {
	case options.save:
		if err := registry.SaveObservations(regPath, installedObservations); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		if err := registry.SaveObservations(registry.AvailabilityPath(regPath), availableObservations); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "saved baseline: %d installed tools, %d available commands\n", len(installedObservations), len(availableObservations))
		writeWarnings(errorOutput, warnings)
	case options.diff:
		if !baselineExists {
			fmt.Fprintln(outputWriter, "no baseline yet — run --save first, then --diff will show what changed")
			writeWarnings(errorOutput, warnings)
			break
		}
		fmt.Fprint(outputWriter, renderObservationDiff(diff))
		if options.available {
			fmt.Fprintln(outputWriter, "AVAILABILITY CHANGES")
			fmt.Fprint(outputWriter, renderObservationDiff(availabilityDiff))
		}
		writeWarnings(errorOutput, warnings)
	case options.json:
		data, err := output.RenderObservationJSON(report)
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintln(outputWriter, string(data))
	case options.list:
		fmt.Fprint(outputWriter, output.RenderObservationTable(report))
	case options.doctor:
		fmt.Fprint(outputWriter, renderDoctorReport(report.AllObservations()))
		writeWarnings(errorOutput, warnings)
	case options.capabilities:
		probeOptions := capabilities.Options{}
		if options.capabilitiesProbe {
			probeOptions.Probe = &scanner.ProbeOptions{Enabled: true}
		}
		results := capabilities.DefaultRegistry().DetectWithOptions(report.AllObservations(), probeOptions)
		data, err := capabilities.RenderJSON(results)
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintln(outputWriter, string(data))
		writeWarnings(errorOutput, warnings)
	case options.snapshot:
		observations := nonHistoryObservations(installedObservations, availableObservations)
		path, err := registry.NewSnapshotStore("").Save(registry.NewSnapshot(observations, appVersion))
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "saved snapshot: %s (%d observations)\n", path, len(observations))
		writeWarnings(errorOutput, warnings)
	case options.export != "":
		value := currentProfile(report, appVersion)
		if err := writeProfile(options.export, value); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "exported profile: %s (%d observations)\n", options.export, len(value.Observations))
		writeWarnings(errorOutput, warnings)
	case options.compare != "":
		before, err := loadProfile(options.compare)
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		diff := profile.CompareProfiles(before, currentProfile(report, appVersion))
		fmt.Fprint(outputWriter, renderObservationDiff(diff))
		writeWarnings(errorOutput, warnings)
	case options.support != "":
		if err := profile.WriteSupportBundle(options.support, currentProfile(report, appVersion)); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
		fmt.Fprintf(outputWriter, "wrote support bundle: %s\n", options.support)
		writeWarnings(errorOutput, warnings)
	default:
		if err := output.RunObservationTUI(report, output.TUIOptions{
			Sources:      registrationSources(registrations),
			RegistryPath: regPath,
			Version:      appVersion,
			Theme:        settings.Theme,
			ConfigPath:   settings.ConfigPath,
			UIMode:       resolveUIMode(settings, options.legacyTabs, options.intentTabs),
		}); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 1
		}
	}
	return 0
}

// resolveUIMode picks the TUI tab layout. Configuration selects it normally;
// the two flags are the no-config-edit overrides for the opt-in intent-tabs
// rollout and both win over the config file. --legacy-tabs is the rollback
// switch, so a user who hits trouble with the new tabs can get the eight-tab
// layout back without editing (or finding) their TOML; --intent-tabs is its
// inverse, the try-it switch for the intent tabs while legacy remains the
// shipped default.
//
// --legacy-tabs is checked first, so it wins if both flags are passed: the
// rollback switch should never be the one that loses.
func resolveUIMode(settings config.Settings, legacyTabs, intentTabs bool) string {
	if legacyTabs {
		return output.UIModeLegacy
	}
	if intentTabs {
		// The output package exports no intent-mode constant; this literal is
		// the value reportTabsForMode compares its unexported uiModeIntent against.
		return "intent"
	}
	return settings.UI.Mode
}

func nonHistoryObservations(installed, available []model.Observation) []model.Observation {
	result := make([]model.Observation, 0, len(installed)+len(available))
	result = append(result, installed...)
	result = append(result, available...)
	return result
}

func listSnapshots(outputWriter, errorOutput io.Writer) int {
	snapshots, err := registry.NewSnapshotStore("").List()
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return 1
	}
	if len(snapshots) == 0 {
		fmt.Fprintln(outputWriter, "no snapshots")
		return 0
	}
	for _, snapshot := range snapshots {
		version := snapshot.Version
		if version == "" {
			version = "unknown"
		}
		fmt.Fprintf(outputWriter, "%s\t%s\t%s\n", snapshot.CreatedAt.UTC().Format(time.RFC3339Nano), version, snapshot.Path)
	}
	return 0
}

func renderDoctorReport(observations []model.Observation) string {
	report := diagnostics.Analyze(observations)
	var b strings.Builder
	fmt.Fprintln(&b, "TOOLSNIFF DOCTOR")
	fmt.Fprintf(&b, "OBSERVATIONS: %d\n", len(observations))
	fmt.Fprintf(&b, "ISSUES: %d\n", len(report.Issues))
	for _, issue := range report.Issues {
		fmt.Fprintf(&b, "  [%s] %s", issue.Kind, issue.Name)
		if issue.CommandName != "" && issue.CommandName != issue.Name {
			fmt.Fprintf(&b, " (%s)", issue.CommandName)
		}
		if detail := diagnosticDetail(issue); detail != "" {
			fmt.Fprintf(&b, ": %s", detail)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "PROVENANCE: %d\n", len(report.Provenance))
	for _, edge := range report.Provenance {
		fmt.Fprintf(&b, "  [%s] %s -> %s (%s)\n", edge.Kind, edge.PackageName, edge.LocationPath, edge.LocationType)
	}
	return b.String()
}

func diagnosticDetail(issue diagnostics.Issue) string {
	switch issue.Kind {
	case diagnostics.IssueShadowedCommand:
		return "shadowed by " + strings.Join(issue.ShadowedPaths, ", ")
	case diagnostics.IssueBrokenLocation:
		return issue.LocationPath
	case diagnostics.IssueArchitectureMismatch:
		return fmt.Sprintf("expected %s, found %s", strings.Join(issue.ExpectedArchitectures, ", "), strings.Join(issue.ObservedArchitectures, ", "))
	case diagnostics.IssueUnsignedApplication:
		return issue.LocationPath
	case diagnostics.IssueUnknownSigningStatus:
		return "signing status unknown"
	case diagnostics.IssueMissingExecutableLink:
		return "missing executable " + issue.ExecutableName
	case diagnostics.IssueUnknownVersion:
		return "version unknown"
	default:
		return ""
	}
}

func currentProfile(report output.ObservationReport, appVersion string) profile.Profile {
	value := profile.New(report.AllObservations(), &report)
	value.Version = appVersion
	return profile.Sanitize(value)
}

func loadProfile(path string) (profile.Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("profile: reading %s: %w", path, err)
	}
	value, err := profile.Unmarshal(data)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("profile: loading %s: %w", path, err)
	}
	return profile.Sanitize(value), nil
}

func writeProfile(path string, value profile.Profile) error {
	if path == "" {
		return errors.New("profile: empty profile path")
	}
	data, err := profile.Marshal(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("profile: creating directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("profile: securing directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".profile-*.tmp")
	if err != nil {
		return fmt.Errorf("profile: creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: securing temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("profile: closing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("profile: replacing %s: %w", path, err)
	}
	return nil
}

func renderObservationDiff(diff registry.ObservationDiff) string {
	if len(diff.Events()) == 0 {
		return "no changes since last scan\n"
	}
	return output.RenderChangeReport(output.NewObservationReport(nil, nil, nil, diff, nil).Changes)
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
