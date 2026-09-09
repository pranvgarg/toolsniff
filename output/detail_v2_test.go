package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestDetailViewIncludesOptionalMetadataSections(t *testing.T) {
	signed := true
	observation := observation("tool", model.RoleInstalled, model.KindApplication, model.VersionInfo{State: model.VersionNotReported, Confidence: model.ConfidenceLow})
	observation.Application = &model.ApplicationInfo{BundleID: "com.example.tool", DisplayVersion: "2.0", Architectures: []string{"arm64"}, Signed: &signed}
	observation.Package = &model.PackageInfo{Name: "tool", Prefix: "/opt/tool", Executables: []string{"tool"}}
	observation.History = &model.HistoryInfo{CachePath: "/tmp/cache"}
	observation.Evidence = []model.Evidence{{Type: "plist", Source: "Info.plist", Description: "bundle metadata"}}
	detail := BuildDetailViewModel(observation)
	output := RenderDetailView(detail)
	for _, want := range []string{"Overview", "Version state", "Locations", "Package", "Application", "History", "Evidence", "com.example.tool", "not reported"} {
		if !strings.Contains(output, want) {
			t.Errorf("detail output missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "Version:\n") || strings.Contains(output, "Version: /opt") {
		t.Fatalf("detail view confused location and version: %s", output)
	}
}

func TestReadOnlyLocationHelpersDoNotExecuteCommands(t *testing.T) {
	observation := observationWithPath("tool", model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/tmp/tool")
	if got := CopySelectedPath(observation); got != "/tmp/tool" {
		t.Fatalf("selected path = %q", got)
	}
	data, err := CopySelectedObservationJSON(observation)
	if err != nil || !strings.Contains(string(data), "\"display_name\": \"tool\"") {
		t.Fatalf("selected JSON = %s, err=%v", data, err)
	}
	command, err := RevealLocationCommand("/tmp/tool")
	if err != nil || strings.Join(command, " ") != "open -R /tmp/tool" {
		t.Fatalf("reveal command = %#v, err=%v", command, err)
	}
}
