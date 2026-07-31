package output

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// tabOrder is the fixed, meaningful order tabs appear in: real sources
// first (in scan-priority order), then npx-history (informational), then
// "new" (only present when there's an actual diff).
var tabOrder = []string{
	"npm", "brew-formula", "brew-cask", "pipx", "cargo",
	"applications", "path", "npx-history",
}

type tuiModel struct {
	tabs       []string
	toolsBySrc map[string][]model.Tool
	activeTab  int
	list       list.Model
	realTools  []model.Tool
	regPath    string
	statusMsg  string
	warnings   []scanner.Warning

	width, height int

	splashPhase splashPhase
	splashLines []string
	splashTimer timer.Model
}

func newTUIModel(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) tuiModel {
	toolsBySrc := map[string][]model.Tool{}
	for _, t := range realTools {
		toolsBySrc[t.Source] = append(toolsBySrc[t.Source], t)
	}
	if len(npxHistory) > 0 {
		toolsBySrc["npx-history"] = npxHistory
	}

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		newTab := append([]model.Tool{}, diff.Added...)
		newTab = append(newTab, diff.Removed...)
		toolsBySrc["new"] = newTab
	}

	tabs := make([]string, 0, len(tabOrder)+1)
	for _, src := range tabOrder {
		if _, ok := toolsBySrc[src]; ok {
			tabs = append(tabs, src)
		}
	}
	if _, ok := toolsBySrc["new"]; ok {
		tabs = append(tabs, "new")
	}
	if len(tabs) == 0 {
		tabs = []string{"npm"}
	}

	l := list.New(itemsFor(toolsBySrc[tabs[0]]), list.NewDefaultDelegate(), 0, 0)
	l.Title = tabs[0]

	return tuiModel{
		tabs:        tabs,
		toolsBySrc:  toolsBySrc,
		list:        l,
		realTools:   realTools,
		regPath:     regPath,
		warnings:    warnings,
		splashTimer: newSplashTimer(),
	}
}

func itemsFor(tools []model.Tool) []list.Item {
	items := make([]list.Item, len(tools))
	for i, t := range tools {
		items[i] = toolItem{tool: t}
	}
	return items
}

func (m tuiModel) Init() tea.Cmd { return m.splashTimer.Init() }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = wsMsg.Width, wsMsg.Height
		m.list.SetSize(wsMsg.Width, wsMsg.Height-4)
		return m, nil
	}

	if m.splashPhase != splashDone {
		return m.updateSplash(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.list.SetItems(itemsFor(m.toolsBySrc[m.tabs[m.activeTab]]))
			m.list.Title = m.tabs[m.activeTab]
			return m, nil
		case "s":
			if err := registry.Save(m.regPath, m.realTools); err != nil {
				m.statusMsg = "save failed: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("saved baseline: %d tools", len(m.realTools))
			}
			return m, nil
		case "d":
			for i, t := range m.tabs {
				if t == "new" {
					m.activeTab = i
					m.list.SetItems(itemsFor(m.toolsBySrc["new"]))
					m.list.Title = "new"
					break
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m tuiModel) View() tea.View {
	if m.splashPhase != splashDone {
		view := tea.NewView(renderSplash(m.splashLines, m.width, m.height))
		view.AltScreen = true
		return view
	}

	tabBar := renderTabBar(m.tabs, m.activeTab, m.toolsBySrc)
	footer := footerStyle.Render("↑↓ move · tab switch · / filter · d diff · s save · q quit")
	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render(m.statusMsg)
	}
	parts := []string{tabBar, m.list.View()}
	for _, w := range m.warnings {
		parts = append(parts, footerStyle.Render(fmt.Sprintf("warning: %s: %v", w.Source, w.Err)))
	}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	view := tea.NewView(strings.Join(parts, "\n"))
	view.AltScreen = true
	return view
}

func renderTabBar(tabs []string, active int, toolsBySrc map[string][]model.Tool) string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%s (%d)", t, len(toolsBySrc[t]))
		if i == active {
			parts[i] = activeTabStyle.Render(label)
		} else {
			parts[i] = tabStyle.Render(label)
		}
	}
	return strings.Join(parts, "  ")
}

// RunTUI launches the interactive Bubbletea program.
func RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error {
	p := tea.NewProgram(newTUIModel(realTools, npxHistory, diff, warnings, regPath))
	_, err := p.Run()
	return err
}
