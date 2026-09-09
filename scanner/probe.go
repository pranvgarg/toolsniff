package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

const (
	// DefaultProbeTimeout keeps opt-in executable inspection bounded.
	DefaultProbeTimeout = 2 * time.Second
	// DefaultProbeMaxOutputBytes prevents a probe from retaining arbitrary output.
	DefaultProbeMaxOutputBytes = 4096
)

// ProbeOptions is supplied by a source-specific adapter. Arguments are passed
// directly to the executable and are never interpreted by a shell.
type ProbeOptions struct {
	Enabled        bool
	Arguments      []string
	Timeout        time.Duration
	MaxOutputBytes int
}

// PathScanOptions controls optional enrichment of PATH evidence.
type PathScanOptions struct {
	Probe *ProbeOptions
}

func (options ProbeOptions) arguments() []string {
	if len(options.Arguments) == 0 {
		return []string{"--version"}
	}
	return append([]string(nil), options.Arguments...)
}

func (options ProbeOptions) limits() (time.Duration, int) {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	maxOutput := options.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = DefaultProbeMaxOutputBytes
	}
	return timeout, maxOutput
}

// ProbeExecutable runs one bounded, direct executable invocation. A disabled
// probe performs no process execution and returns the normal not-reported
// version state. Probe failures are returned as evidence, not scanner errors.
func ProbeExecutable(path string, options ProbeOptions) (model.ProbeResult, model.VersionInfo) {
	version := model.VersionInfo{
		State:       model.VersionNotReported,
		Scheme:      model.SchemeUnknown,
		Confidence:  model.ConfidenceLow,
		RetrievedBy: "probe",
	}
	if !options.Enabled {
		return model.ProbeResult{}, version
	}

	retrievedAt := time.Now().UTC()
	result := model.ProbeResult{RetrievedAt: &retrievedAt}
	timeout, maxOutput := options.limits()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, options.arguments()...)
	capture := newProbeCapture(maxOutput)
	cmd.Stdout = capture.writer(false)
	cmd.Stderr = capture.writer(true)
	err := cmd.Run()
	if ctx.Err() != nil {
		result.Error = "probe timed out"
		version.State = model.VersionUnknown
		return result, version
	}
	if capture.truncatedOutput() {
		result.Error = fmt.Sprintf("probe output exceeded %d bytes", maxOutput)
		version.State = model.VersionUnknown
		return result, version
	}
	if err != nil {
		result.Error = err.Error()
		version.State = model.VersionUnknown
		return result, version
	}

	value := firstProbeLine(capture.stdoutBytes())
	if value == "" {
		result.Error = "probe returned no version"
		version.State = model.VersionUnknown
		return result, version
	}
	result.Version = value
	version.Value = value
	version.State = model.VersionKnown
	version.Scheme = model.SchemeOpaque
	version.Confidence = model.ConfidenceMedium
	return result, version
}

func firstProbeLine(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

type probeCapture struct {
	mu        sync.Mutex
	limit     int
	total     int
	truncated bool
	stdout    bytes.Buffer
}

func newProbeCapture(limit int) *probeCapture { return &probeCapture{limit: limit} }

func (capture *probeCapture) writer(stderr bool) *probeStream {
	return &probeStream{capture: capture, stderr: stderr}
}

func (capture *probeCapture) write(stderr bool, data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	remaining := capture.limit - capture.total
	if remaining <= 0 {
		capture.truncated = true
		return len(data), nil
	}
	accepted := len(data)
	if accepted > remaining {
		accepted = remaining
		capture.truncated = true
	}
	capture.total += accepted
	if !stderr {
		_, _ = capture.stdout.Write(data[:accepted])
	}
	return len(data), nil
}

func (capture *probeCapture) stdoutBytes() []byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.stdout.Bytes()...)
}

func (capture *probeCapture) truncatedOutput() bool {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.truncated
}

type probeStream struct {
	capture *probeCapture
	stderr  bool
}

func (stream *probeStream) Write(data []byte) (int, error) {
	return stream.capture.write(stream.stderr, data)
}
