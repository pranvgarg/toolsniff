package registry

import (
	"reflect"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ChangeKind identifies the kind of change represented by a ChangeEvent.
type ChangeKind string

const (
	ChangeAdded     ChangeKind = "added"
	ChangeRemoved   ChangeKind = "removed"
	ChangeUpdated   ChangeKind = "updated"
	ChangeRelocated ChangeKind = "relocated"
	ChangeBroken    ChangeKind = "broken"
	ChangeRepaired  ChangeKind = "repaired"
	ChangeShadowed  ChangeKind = "shadowed"
)

// ChangeEvent records one change to an observation identity. Added events have
// only After, removed events have only Before, and state changes have both.
type ChangeEvent struct {
	Kind     ChangeKind         `json:"kind"`
	Identity string             `json:"identity"`
	Before   *model.Observation `json:"before,omitempty"`
	After    *model.Observation `json:"after,omitempty"`
}

// ObservationDiff is the observation-based counterpart to the legacy Diff.
// Each category is sorted by stable identity and then by observation details.
type ObservationDiff struct {
	Added     []ChangeEvent `json:"added"`
	Removed   []ChangeEvent `json:"removed"`
	Updated   []ChangeEvent `json:"updated"`
	Relocated []ChangeEvent `json:"relocated"`
	Broken    []ChangeEvent `json:"broken"`
	Repaired  []ChangeEvent `json:"repaired"`
	Shadowed  []ChangeEvent `json:"shadowed"`
}

// ComputeObservationDiff compares observation identities while keeping their
// locations and availability states separate from identity matching.
func ComputeObservationDiff(old, new []model.Observation) ObservationDiff {
	diff := ObservationDiff{
		Added:     []ChangeEvent{},
		Removed:   []ChangeEvent{},
		Updated:   []ChangeEvent{},
		Relocated: []ChangeEvent{},
		Broken:    []ChangeEvent{},
		Repaired:  []ChangeEvent{},
		Shadowed:  []ChangeEvent{},
	}

	oldByIdentity := observationsByIdentity(old)
	newByIdentity := observationsByIdentity(new)

	for _, identity := range sortedObservationIdentities(newByIdentity) {
		current := newByIdentity[identity]
		previous, ok := oldByIdentity[identity]
		if !ok {
			diff.Added = append(diff.Added, newEvent(ChangeAdded, identity, current))
			continue
		}

		if versionChanged(previous.Version, current.Version) || metadataChanged(previous, current) {
			diff.Updated = append(diff.Updated, pairedEvent(ChangeUpdated, identity, previous, current))
		}
		if locationsChanged(previous, current) {
			diff.Relocated = append(diff.Relocated, pairedEvent(ChangeRelocated, identity, previous, current))
		}
		if previous.Availability.State != model.AvailabilityUnavailable && current.Availability.State == model.AvailabilityUnavailable {
			diff.Broken = append(diff.Broken, pairedEvent(ChangeBroken, identity, previous, current))
		}
		if previous.Availability.State == model.AvailabilityUnavailable && current.Availability.State == model.AvailabilityAvailable {
			diff.Repaired = append(diff.Repaired, pairedEvent(ChangeRepaired, identity, previous, current))
		}
		if shadowingChanged(previous, current) && isShadowed(current) {
			diff.Shadowed = append(diff.Shadowed, pairedEvent(ChangeShadowed, identity, previous, current))
		}
	}

	for _, identity := range sortedObservationIdentities(oldByIdentity) {
		if _, ok := newByIdentity[identity]; !ok {
			diff.Removed = append(diff.Removed, removedEvent(ChangeRemoved, identity, oldByIdentity[identity]))
		}
	}

	sortChangeEvents(diff.Added)
	sortChangeEvents(diff.Removed)
	sortChangeEvents(diff.Updated)
	sortChangeEvents(diff.Relocated)
	sortChangeEvents(diff.Broken)
	sortChangeEvents(diff.Repaired)
	sortChangeEvents(diff.Shadowed)
	return diff
}

// Events returns all changes in a stable category order. The category slices
// remain available for report formats that need separate arrays.
func (d ObservationDiff) Events() []ChangeEvent {
	events := make([]ChangeEvent, 0, len(d.Added)+len(d.Removed)+len(d.Updated)+len(d.Relocated)+len(d.Broken)+len(d.Repaired)+len(d.Shadowed))
	events = append(events, d.Added...)
	events = append(events, d.Removed...)
	events = append(events, d.Updated...)
	events = append(events, d.Relocated...)
	events = append(events, d.Broken...)
	events = append(events, d.Repaired...)
	events = append(events, d.Shadowed...)
	return events
}

func observationsByIdentity(observations []model.Observation) map[string]model.Observation {
	byIdentity := make(map[string]model.Observation, len(observations))
	for _, observation := range observations {
		byIdentity[model.ObservationIdentity(observation)] = observation
	}
	return byIdentity
}

func sortedObservationIdentities(observations map[string]model.Observation) []string {
	identities := make([]string, 0, len(observations))
	for identity := range observations {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	return identities
}

func newEvent(kind ChangeKind, identity string, observation model.Observation) ChangeEvent {
	return ChangeEvent{Kind: kind, Identity: identity, After: &observation}
}

func removedEvent(kind ChangeKind, identity string, observation model.Observation) ChangeEvent {
	return ChangeEvent{Kind: kind, Identity: identity, Before: &observation}
}

func pairedEvent(kind ChangeKind, identity string, before, after model.Observation) ChangeEvent {
	return ChangeEvent{Kind: kind, Identity: identity, Before: &before, After: &after}
}

// A version becomes an update when a known value changes or when a previously
// non-known state becomes known. Losing a reported version is not a change;
// this avoids noisy updates caused by transient probe failures.
func versionChanged(before, after model.VersionInfo) bool {
	if before.State == model.VersionKnown && after.State == model.VersionKnown {
		return before != after
	}
	return before.State != model.VersionKnown && after.State == model.VersionKnown
}

func metadataChanged(before, after model.Observation) bool {
	before.ID = ""
	after.ID = ""
	before.Locations = nil
	after.Locations = nil
	before.Availability = model.AvailabilityInfo{}
	after.Availability = model.AvailabilityInfo{}
	before.Version = model.VersionInfo{}
	after.Version = model.VersionInfo{}
	before.Evidence = nil
	after.Evidence = nil
	before.FirstSeen = nil
	after.FirstSeen = nil
	before.LastSeen = nil
	after.LastSeen = nil
	return !reflect.DeepEqual(before, after)
}

func locationsChanged(before, after model.Observation) bool {
	beforeKeys := locationKeys(before.Locations)
	afterKeys := locationKeys(after.Locations)
	return !reflect.DeepEqual(beforeKeys, afterKeys)
}

func locationKeys(locations []model.Location) []string {
	keys := make([]string, 0, len(locations))
	for _, location := range locations {
		keys = append(keys, string(location.Type)+"\x00"+location.Path)
	}
	sort.Strings(keys)
	return keys
}

func isShadowed(observation model.Observation) bool {
	return len(observation.Availability.ShadowedBy) > 0
}

func shadowingChanged(before, after model.Observation) bool {
	beforeShadowed := append([]string(nil), before.Availability.ShadowedBy...)
	afterShadowed := append([]string(nil), after.Availability.ShadowedBy...)
	sort.Strings(beforeShadowed)
	sort.Strings(afterShadowed)
	return !reflect.DeepEqual(beforeShadowed, afterShadowed)
}

func sortChangeEvents(events []ChangeEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Identity != events[j].Identity {
			return events[i].Identity < events[j].Identity
		}
		return eventObservationKey(events[i]) < eventObservationKey(events[j])
	})
}

func eventObservationKey(event ChangeEvent) string {
	observation := event.After
	if observation == nil {
		observation = event.Before
	}
	if observation == nil {
		return ""
	}
	locations := locationKeys(observation.Locations)
	return string(observation.Kind) + "\x00" + observation.DisplayName + "\x00" + observation.CommandName + "\x00" + strings.Join(locations, "\x00")
}
