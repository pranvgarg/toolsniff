package registry

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestComputeObservationDiffIdentityAndStateChanges(t *testing.T) {
	known := model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh}
	tests := []struct {
		name         string
		before       model.Observation
		after        model.Observation
		added        int
		removed      int
		updated      int
		relocated    int
		broken       int
		repaired     int
		shadowed     int
		expectedPath string
	}{
		{
			name:         "package relocation",
			before:       packageObservation("npm", "global", "tool", "/usr/local/lib/tool"),
			after:        packageObservation("npm", "global", "tool", "/opt/lib/tool"),
			relocated:    1,
			expectedPath: "/opt/lib/tool",
		},
		{
			name:         "application bundle identity",
			before:       applicationObservation("com.example.tool", "/Applications/Tool.app"),
			after:        applicationObservation("com.example.tool", "/Volumes/Tools/Tool.app"),
			relocated:    1,
			expectedPath: "/Volumes/Tools/Tool.app",
		},
		{
			name:         "PATH addition and removal",
			before:       pathObservation("/usr/local/bin/tool"),
			after:        pathObservation("/opt/bin/tool"),
			added:        1,
			removed:      1,
			expectedPath: "/opt/bin/tool",
		},
		{
			name:    "known version update",
			before:  packageObservationWithVersion("npm", "global", "tool", "/opt/lib/tool", known),
			after:   packageObservationWithVersion("npm", "global", "tool", "/opt/lib/tool", model.VersionInfo{Value: "2.0.0", State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh}),
			updated: 1,
		},
		{
			name:   "broken location",
			before: withAvailability(packageObservation("npm", "global", "tool", "/opt/lib/tool"), model.AvailabilityInfo{State: model.AvailabilityAvailable}),
			after:  withAvailability(packageObservation("npm", "global", "tool", "/opt/lib/tool"), model.AvailabilityInfo{State: model.AvailabilityUnavailable}),
			broken: 1,
		},
		{
			name:     "repaired location",
			before:   withAvailability(packageObservation("npm", "global", "tool", "/opt/lib/tool"), model.AvailabilityInfo{State: model.AvailabilityUnavailable}),
			after:    withAvailability(packageObservation("npm", "global", "tool", "/opt/lib/tool"), model.AvailabilityInfo{State: model.AvailabilityAvailable}),
			repaired: 1,
		},
		{
			name:   "shadowed availability",
			before: withAvailability(pathObservation("/usr/local/bin/tool"), model.AvailabilityInfo{State: model.AvailabilityAvailable}),
			after: withAvailability(pathObservation("/usr/local/bin/tool"), model.AvailabilityInfo{
				State:      model.AvailabilityAvailable,
				ShadowedBy: []string{"/opt/bin/tool"},
			}),
			shadowed: 1,
		},
		{
			name:   "unchanged observation",
			before: packageObservation("npm", "global", "tool", "/opt/lib/tool"),
			after:  packageObservation("npm", "global", "tool", "/opt/lib/tool"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := ComputeObservationDiff([]model.Observation{test.before}, []model.Observation{test.after})
			if len(diff.Added) != test.added || len(diff.Removed) != test.removed || len(diff.Updated) != test.updated || len(diff.Relocated) != test.relocated || len(diff.Broken) != test.broken || len(diff.Repaired) != test.repaired || len(diff.Shadowed) != test.shadowed {
				t.Fatalf("unexpected diff: %+v", diff)
			}
			if test.expectedPath != "" && test.relocated == 1 {
				if got := diff.Relocated[0].After.Locations[0].Path; got != test.expectedPath {
					t.Fatalf("relocation path = %q, want %q", got, test.expectedPath)
				}
			}
		})
	}
}

func TestComputeObservationDiffDeterministicOrderingAndVersionStates(t *testing.T) {
	added := ComputeObservationDiff(nil, []model.Observation{
		packageObservation("npm", "global", "z-tool", "/z"),
		packageObservation("npm", "global", "a-tool", "/a"),
	})
	if len(added.Added) != 2 || added.Added[0].Identity >= added.Added[1].Identity {
		t.Fatalf("added events are not sorted: %+v", added.Added)
	}

	old := []model.Observation{
		packageObservation("npm", "global", "z-tool", "/z"),
		packageObservation("npm", "global", "a-tool", "/a"),
	}
	current := []model.Observation{
		packageObservation("npm", "global", "z-tool", "/z"),
		packageObservationWithVersion("npm", "global", "a-tool", "/a", model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh}),
	}

	diff := ComputeObservationDiff(old, current)
	if len(diff.Updated) != 1 || diff.Updated[0].Identity != model.ObservationIdentity(current[1]) {
		t.Fatalf("expected known version to be reported: %+v", diff.Updated)
	}
	if events := diff.Events(); len(events) != 1 || events[0].Kind != ChangeUpdated {
		t.Fatalf("unexpected event stream: %+v", events)
	}

	known := packageObservationWithVersion("npm", "global", "tool", "/tool", model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh})
	unknown := known
	unknown.Version = model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}
	if diff := ComputeObservationDiff([]model.Observation{known}, []model.Observation{unknown}); len(diff.Updated) != 0 {
		t.Fatalf("known-to-unknown should not be an update: %+v", diff.Updated)
	}
}

func packageObservation(provider, manager, name, path string) model.Observation {
	return packageObservationWithVersion(provider, manager, name, path, model.VersionInfo{State: model.VersionNotReported, Confidence: model.ConfidenceLow})
}

func packageObservationWithVersion(provider, manager, name, path string, version model.VersionInfo) model.Observation {
	observation := model.Observation{
		DisplayName: name,
		CommandName: name,
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: provider, Manager: manager, Package: name},
		Version:     version,
		Locations:   []model.Location{{Path: path, Type: model.LocationPackagePrefix}},
		Availability: model.AvailabilityInfo{
			State: model.AvailabilityUnknown,
		},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

func applicationObservation(bundleID, path string) model.Observation {
	observation := model.Observation{
		DisplayName: nameFromPath(path),
		Kind:        model.KindApplication,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "applications"},
		Version:     model.VersionInfo{State: model.VersionNotReported, Confidence: model.ConfidenceLow},
		Locations:   []model.Location{{Path: path, Type: model.LocationApplication}},
		Availability: model.AvailabilityInfo{
			State: model.AvailabilityUnknown,
		},
		Application: &model.ApplicationInfo{BundleID: bundleID},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

func pathObservation(path string) model.Observation {
	observation := model.Observation{
		DisplayName: nameFromPath(path),
		CommandName: nameFromPath(path),
		Kind:        model.KindExecutable,
		Role:        model.RoleAvailable,
		Origin:      model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
		Version:     model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow},
		Locations:   []model.Location{{Path: path, Type: model.LocationExecutable, Executable: true}},
		Availability: model.AvailabilityInfo{
			State:        model.AvailabilityAvailable,
			ResolvedPath: path,
		},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

func withAvailability(observation model.Observation, availability model.AvailabilityInfo) model.Observation {
	observation.Availability = availability
	return observation
}

func nameFromPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
