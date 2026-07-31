package output

import (
	"math/rand"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// version is the toolsniff version shown on the splash screen. It will
// likely be wired to a real --version flag later.
const version = "0.1.0"

// splashPhase models where the splash screen is in its lifecycle.
type splashPhase int

const (
	splashHold splashPhase = iota
	splashDissolve1
	splashDissolve2
	splashDone
)

const (
	splashHoldDuration     = 1500 * time.Millisecond
	splashDissolveInterval = 90 * time.Millisecond

	splashDissolveProb1 = 0.35
	splashDissolveProb2 = 0.75
)

// dissolveChars are the characters used, alongside a plain space, when a
// wordmark glyph "dissolves" during the transition off the splash screen.
var dissolveChars = []rune{'░', '·'}

// wordmarkLines is the ASCII-art "TOOLSNIFF" wordmark rendered on the
// splash screen.
var wordmarkLines = []string{
	`      ████████╗ ██████╗  ██████╗ ██╗         ███████╗███╗   ██╗██╗`,
	`      ╚══██╔══╝██╔═══██╗██╔═══██╗██║         ██╔════╝████╗  ██║██║`,
	`         ██║   ██║   ██║██║   ██║██║         ███████╗██╔██╗ ██║██║`,
	`         ██║   ██║   ██║██║   ██║██║         ╚════██║██║╚██╗██║██║`,
	`         ██║   ╚██████╔╝╚██████╔╝███████╗    ███████║██║ ╚████║██║`,
	`         ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝    ╚══════╝╚═╝  ╚═══╝╚═╝`,
}

var (
	wordmarkStyle = lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	versionStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	splashBorder  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorMuted)
)

// splashDissolveTickMsg fires on each dissolve animation frame.
type splashDissolveTickMsg struct{}

// newSplashTimer builds the timer.Model that drives the splash screen's
// hold phase. Its interval is set to the full hold duration so it fires
// exactly once, delivering a timer.TimeoutMsg at ~splashHoldDuration.
func newSplashTimer() timer.Model {
	return timer.New(splashHoldDuration, timer.WithInterval(splashHoldDuration))
}

func splashDissolveCmd() tea.Cmd {
	return tea.Tick(splashDissolveInterval, func(time.Time) tea.Msg {
		return splashDissolveTickMsg{}
	})
}

// dissolveWordmark returns a copy of wordmarkLines where each non-space
// rune has, independently, a prob chance of being replaced by a space or a
// dim placeholder character, simulating the wordmark breaking apart.
func dissolveWordmark(prob float64) []string {
	out := make([]string, len(wordmarkLines))
	for i, line := range wordmarkLines {
		runes := []rune(line)
		for j, r := range runes {
			if r == ' ' {
				continue
			}
			if rand.Float64() < prob {
				if rand.Intn(2) == 0 {
					runes[j] = ' '
				} else {
					runes[j] = dissolveChars[rand.Intn(len(dissolveChars))]
				}
			}
		}
		out[i] = string(runes)
	}
	return out
}

// updateSplash handles messages while the splash screen is active. Any
// keypress immediately dismisses the splash and consumes the event, so it
// is never also interpreted as a normal TUI keybinding.
func (m tuiModel) updateSplash(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.splashPhase = splashDone
		m.splashLines = nil
		return m, nil
	case timer.TickMsg:
		if m.splashPhase != splashHold {
			return m, nil
		}
		var cmd tea.Cmd
		m.splashTimer, cmd = m.splashTimer.Update(msg)
		return m, cmd
	case timer.TimeoutMsg:
		if m.splashPhase != splashHold {
			return m, nil
		}
		m.splashPhase = splashDissolve1
		m.splashLines = dissolveWordmark(splashDissolveProb1)
		return m, splashDissolveCmd()
	case splashDissolveTickMsg:
		switch m.splashPhase {
		case splashDissolve1:
			m.splashPhase = splashDissolve2
			m.splashLines = dissolveWordmark(splashDissolveProb2)
			return m, splashDissolveCmd()
		case splashDissolve2:
			m.splashPhase = splashDone
			m.splashLines = nil
			return m, nil
		}
	}
	return m, nil
}

// renderSplash draws the splash frame: a sharp-cornered border matching
// the terminal size, containing the (possibly dissolving) wordmark and
// version line, centered both horizontally and vertically.
func renderSplash(lines []string, width, height int) string {
	if lines == nil {
		lines = wordmarkLines
	}
	if width <= 0 || height <= 0 {
		// No window size known yet; render unplaced so something shows up
		// on the very first frame before a tea.WindowSizeMsg arrives.
		width, height = 80, 24
	}

	styledWordmark := make([]string, len(lines))
	for i, l := range lines {
		styledWordmark[i] = wordmarkStyle.Render(l)
	}
	wordmark := lipgloss.JoinVertical(lipgloss.Left, styledWordmark...)

	versionLine := versionStyle.Render("toolsniff  v" + version)

	block := lipgloss.JoinVertical(lipgloss.Center, wordmark, "", versionLine)

	inner := lipgloss.Place(width-2, height-2, lipgloss.Center, lipgloss.Center, block)
	return splashBorder.Width(width - 2).Height(height - 2).Render(inner)
}
