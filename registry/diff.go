package registry

import (
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// Diff describes what changed between a saved baseline and a fresh scan.
type Diff struct {
	Added   []model.Tool
	Removed []model.Tool
	Updated []ToolChange
}

// ToolChange records an observation whose identity remained stable while its
// metadata changed.
type ToolChange struct {
	Before model.Tool `json:"before"`
	After  model.Tool `json:"after"`
}

func toolKey(t model.Tool) string { return model.ToolIdentity(t) }

func isVersionUpdate(before, after model.Tool) bool {
	if before.Version == after.Version {
		return false
	}
	return after.Version != ""
}

// ComputeDiff compares an old baseline against a new scan, keyed on
// (Source, Path) when a path is available, otherwise (Source, Name), so two
// installation sources and two distinct executable locations remain visible.
func ComputeDiff(old, new []model.Tool) Diff {
	oldSet := make(map[string]model.Tool, len(old))
	for _, t := range old {
		oldSet[toolKey(t)] = t
	}
	newSet := make(map[string]model.Tool, len(new))
	for _, t := range new {
		newSet[toolKey(t)] = t
	}

	var added, removed []model.Tool
	var updated []ToolChange
	for key, t := range newSet {
		before, ok := oldSet[key]
		if !ok {
			added = append(added, t)
		} else if isVersionUpdate(before, t) {
			updated = append(updated, ToolChange{Before: before, After: t})
		}
	}
	for key, t := range oldSet {
		if _, ok := newSet[key]; !ok {
			removed = append(removed, t)
		}
	}

	byBothKeys := func(tools []model.Tool) func(i, j int) bool {
		return func(i, j int) bool {
			if tools[i].Source != tools[j].Source {
				return tools[i].Source < tools[j].Source
			}
			return tools[i].Name < tools[j].Name
		}
	}
	sort.Slice(added, byBothKeys(added))
	sort.Slice(removed, byBothKeys(removed))
	sort.Slice(updated, func(i, j int) bool {
		left, right := updated[i], updated[j]
		if left.After.Source != right.After.Source {
			return left.After.Source < right.After.Source
		}
		if left.After.Name != right.After.Name {
			return left.After.Name < right.After.Name
		}
		return left.After.Path < right.After.Path
	})
	return Diff{Added: added, Removed: removed, Updated: updated}
}
