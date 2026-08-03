package scanner

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

func TestMain(m *testing.M) {
	switch os.Getenv("TOOL_SNIFF_PROBE_HELPER") {
	case "success":
		fmt.Fprintln(os.Stdout, os.Args[len(os.Args)-1])
		os.Exit(0)
	case "non-zero":
		fmt.Fprintln(os.Stdout, "ignored-version")
		os.Exit(7)
	case "timeout":
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "output-limit":
		_, _ = os.Stdout.Write(bytes.Repeat([]byte{'x'}, 256))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestProbeExecutableSuccessUsesFixedArguments(t *testing.T) {
	result, version := runProbe(t, "success", ProbeOptions{
		Enabled:        true,
		Arguments:      []string{"version", "literal-$()"},
		Timeout:        5 * time.Second,
		MaxOutputBytes: 128,
	})
	if result.Error != "" || result.Version != "literal-$()" {
		t.Fatalf("unexpected probe result: %+v", result)
	}
	if version.State != model.VersionKnown || version.Value != "literal-$()" || version.RetrievedBy != "probe" {
		t.Fatalf("unexpected version: %+v", version)
	}
}

func TestProbeExecutableTimeoutIsBounded(t *testing.T) {
	started := time.Now()
	result, version := runProbe(t, "timeout", ProbeOptions{Enabled: true, Timeout: 40 * time.Millisecond, MaxOutputBytes: 128})
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("probe was not bounded: %v", elapsed)
	}
	if result.Error != "probe timed out" || version.State != model.VersionUnknown {
		t.Fatalf("unexpected timeout result: result=%+v version=%+v", result, version)
	}
}

func TestProbeExecutableNonZeroExitIsEvidence(t *testing.T) {
	result, version := runProbe(t, "non-zero", ProbeOptions{Enabled: true, Timeout: time.Second, MaxOutputBytes: 128})
	if result.Error == "" || version.State != model.VersionUnknown || version.Value != "" {
		t.Fatalf("unexpected non-zero result: result=%+v version=%+v", result, version)
	}
}

func TestProbeExecutableLimitsCombinedOutput(t *testing.T) {
	result, version := runProbe(t, "output-limit", ProbeOptions{Enabled: true, Timeout: 5 * time.Second, MaxOutputBytes: 32})
	if result.Error != "probe output exceeded 32 bytes" || version.State != model.VersionUnknown {
		t.Fatalf("unexpected output-limit result: result=%+v version=%+v", result, version)
	}
}

func TestProbeExecutableIsOptIn(t *testing.T) {
	result, version := ProbeExecutable("/definitely/not/run", ProbeOptions{})
	if result != (model.ProbeResult{}) {
		t.Fatalf("disabled probe executed or recorded evidence: %+v", result)
	}
	if version.State != model.VersionNotReported || version.RetrievedBy != "probe" {
		t.Fatalf("unexpected disabled version: %+v", version)
	}
}

func runProbe(t *testing.T, helper string, options ProbeOptions) (model.ProbeResult, model.VersionInfo) {
	t.Helper()
	t.Setenv("TOOL_SNIFF_PROBE_HELPER", helper)
	options.Arguments = append([]string{"probe"}, options.Arguments...)
	return ProbeExecutable(os.Args[0], options)
}
