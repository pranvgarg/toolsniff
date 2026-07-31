// main.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func buildScanners() []scanner.Scanner {
	return []scanner.Scanner{
		scanner.NewNPMScanner(scanner.ExecRunner),
		scanner.NewNPXScanner(scanner.DefaultNPXDir()),
		scanner.NewHomebrewFormulaScanner(scanner.ExecRunner),
		scanner.NewHomebrewCaskScanner(scanner.ExecRunner),
		scanner.NewPipxScanner(scanner.ExecRunner),
		scanner.NewCargoScanner(scanner.DefaultCargoBinDir()),
		scanner.NewApplicationsScanner(scanner.DefaultApplicationsDir(), scanner.DefaultApplicationKeywords()),
		scanner.NewPathScanner(exec.LookPath, scanner.DefaultPathCandidates()),
	}
}

func splitNPXHistory(tools []model.Tool) (real, npxHistory []model.Tool) {
	for _, t := range tools {
		if t.Source == "npx-history" {
			npxHistory = append(npxHistory, t)
		} else {
			real = append(real, t)
		}
	}
	return real, npxHistory
}

func main() {
	listFlag := flag.Bool("list", false, "print a plain grouped table and exit")
	jsonFlag := flag.Bool("json", false, "print the full scan as JSON and exit")
	saveFlag := flag.Bool("save", false, "scan, save the result as the new baseline, and exit")
	diffFlag := flag.Bool("diff", false, "scan, print only what changed since the last save, and exit")
	flag.Parse()

	tools, warnings := scanner.RunAll(buildScanners())
	realTools, npxHistory := splitNPXHistory(tools)

	regPath := registry.DefaultRegistryPath()
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

	case *diffFlag:
		fmt.Print(output.RenderDiff(diff))

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
		if err := output.RunTUI(realTools, npxHistory, diff, warnings, regPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
