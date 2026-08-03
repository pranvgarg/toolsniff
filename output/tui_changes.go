package output

import (
	"fmt"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// Events returns typed events in the stable report category order.
func (c ChangeReport) Events() []registry.ChangeEvent {
	events := make([]registry.ChangeEvent, 0)
	events = append(events, c.Added...)
	events = append(events, c.Removed...)
	events = append(events, c.Updated...)
	events = append(events, c.Relocated...)
	events = append(events, c.Broken...)
	events = append(events, c.Repaired...)
	events = append(events, c.Shadowed...)
	return events
}

// RenderChangeReport preserves event type and before/after context instead of
// converting changes to ordinary inventory rows.
func RenderChangeReport(changes ChangeReport) string {
	var b strings.Builder
	render := func(title string, events []registry.ChangeEvent) {
		if len(events) == 0 {
			return
		}
		fmt.Fprintln(&b, title)
		for _, event := range events {
			fmt.Fprint(&b, renderChangeEvent(event))
		}
	}
	render("ADDED", changes.Added)
	render("REMOVED", changes.Removed)
	render("UPDATED", changes.Updated)
	render("RELOCATED", changes.Relocated)
	render("BROKEN", changes.Broken)
	render("REPAIRED", changes.Repaired)
	render("SHADOWED", changes.Shadowed)
	return b.String()
}

// RenderChangeEvents renders an event slice while retaining its typed kind.
func RenderChangeEvents(events []registry.ChangeEvent) string {
	var b strings.Builder
	for _, event := range events {
		fmt.Fprintf(&b, "%s\n", strings.ToUpper(string(event.Kind)))
		b.WriteString(renderChangeEvent(event))
	}
	return b.String()
}

func renderChangeEvent(event registry.ChangeEvent) string {
	observation := event.After
	if observation == nil {
		observation = event.Before
	}
	if observation == nil {
		return fmt.Sprintf("  %s %s\n", strings.ToUpper(string(event.Kind)), event.Identity)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "  %s\n", observation.DisplayName)
	switch event.Kind {
	case registry.ChangeUpdated:
		fmt.Fprintf(&b, "    %s -> %s\n", versionForEvent(event.Before), versionForEvent(event.After))
	case registry.ChangeRelocated:
		fmt.Fprintf(&b, "    %s\n    -> %s\n", locationForEvent(event.Before), locationForEvent(event.After))
	case registry.ChangeBroken, registry.ChangeRepaired:
		fmt.Fprintf(&b, "    %s\n", locationForEvent(observation))
	case registry.ChangeShadowed:
		fmt.Fprintf(&b, "    active: %s\n", locationForEvent(observation))
		for _, location := range shadowedLocations(observation) {
			fmt.Fprintf(&b, "    hidden: %s\n", location)
		}
	case registry.ChangeAdded, registry.ChangeRemoved:
		fmt.Fprintf(&b, "    %s\n", ObservationSource(*observation))
	default:
		fmt.Fprintf(&b, "    %s\n", string(event.Kind))
	}
	return b.String()
}

func versionForEvent(observation *model.Observation) string {
	if observation == nil {
		return "n/a"
	}
	return DisplayVersion(observation.Version)
}

func locationForEvent(observation *model.Observation) string {
	if observation == nil || len(observation.Locations) == 0 {
		return "location unavailable"
	}
	return observation.Locations[0].Path
}

func shadowedLocations(observation *model.Observation) []string {
	if observation == nil {
		return nil
	}
	return append([]string(nil), observation.Availability.ShadowedBy...)
}
