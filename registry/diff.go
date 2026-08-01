package registry

import (
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// Diff describes what changed between a saved baseline and a fresh scan.
type Diff struct {
	Added   []model.Tool
	Removed []model.Tool
}

func toolKey(t model.Tool) string { return model.ToolIdentity(t) }

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
	for key, t := range newSet {
		if _, ok := oldSet[key]; !ok {
			added = append(added, t)
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
	return Diff{Added: added, Removed: removed}
}
