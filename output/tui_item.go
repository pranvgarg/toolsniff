package output

import "github.com/pranvgarg/toolsniff/model"

// toolItem adapts model.Tool to bubbles/list's Item interface.
type toolItem struct {
	tool model.Tool
}

func (i toolItem) Title() string { return i.tool.Name }

func (i toolItem) Description() string {
	if i.tool.Version != "" {
		return i.tool.Version
	}
	return i.tool.Path
}

func (i toolItem) FilterValue() string { return i.tool.Name }
