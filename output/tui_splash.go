package output

import (
	"math/rand"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
	`████████╗ ██████╗  ██████╗ ██╗         ███████╗███╗   ██╗██╗███████╗███████╗`,
	`╚══██╔══╝██╔═══██╗██╔═══██╗██║         ██╔════╝████╗  ██║██║██╔════╝██╔════╝`,
	`   ██║   ██║   ██║██║   ██║██║         ███████╗██╔██╗ ██║██║█████╗  █████╗`,
	`   ██║   ██║   ██║██║   ██║██║         ╚════██║██║╚██╗██║██║██╔══╝  ██╔══╝`,
	`   ██║   ╚██████╔╝╚██████╔╝███████╗    ███████║██║ ╚████║██║██║     ██║`,
	`   ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝    ╚══════╝╚═╝  ╚═══╝╚═╝╚═╝     ╚═╝`,
}

// compactWordmarkLines is the condensed lockup used between the plain-text
// fallback and the full wordmark. The full "TOOLSNIFF" block needs 76 columns
// plus the frame's 2, which an 80-column terminal only just misses once any
// terminal padding is involved; this form fits comfortably at 80 so a standard
// window still gets branding instead of the bare text fallback.
var compactWordmarkLines = []string{
	`╭─╮╭─╮╭─╮╷   ╭─╮╭╮╷╷╭─╴╭─╴`,
	`│ ││ ││ ││   ╰─╮│╰┤│├─╴├─╴`,
	`╰─╯╰─╯╰─╯╰─╴ ╰─╯╵ ╵╵╵  ╵  `,
}

// dissolveFraction estimates how far a dissolving wordmark has progressed by
// comparing it against the original, so a differently-sized lockup can be
// dissolved to the same degree without threading animation state through the
// render signature.
func dissolveFraction(lines []string) float64 {
	if len(lines) != len(wordmarkLines) {
		return 0
	}
	var total, gone float64
	for i, line := range lines {
		original, current := []rune(wordmarkLines[i]), []rune(line)
		for j, r := range original {
			if r == ' ' {
				continue
			}
			total++
			if j >= len(current) || current[j] != r {
				gone++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return gone / total
}

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
	return dissolveLines(wordmarkLines, prob)
}

func dissolveLines(source []string, prob float64) []string {
	if prob <= 0 {
		return source
	}
	out := make([]string, len(source))
	for i, line := range source {
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
func renderSplash(lines []string, width, height int, version string, styles ThemeStyles) string {
	if lines == nil {
		lines = wordmarkLines
	}
	if width <= 0 || height <= 0 {
		// No window size known yet; render unplaced so something shows up
		// on the very first frame before a tea.WindowSizeMsg arrives.
		width, height = 80, 24
	}

	if version == "" {
		version = "dev"
	}
	versionLine := styles.Version.Render("toolsniff  v" + version)

	var block string
	if width-2 < wordmarkWidth() && width-2 >= linesWidth(compactWordmarkLines) {
		// Too narrow for the full ASCII lockup but wide enough for the
		// condensed one: an 80-column terminal should still get a branded
		// splash rather than dropping straight to the text fallback.
		source := dissolveLines(compactWordmarkLines, dissolveFraction(lines))
		styledWordmark := make([]string, len(source))
		for i, l := range source {
			styledWordmark[i] = styles.Wordmark.Render(l)
		}
		block = lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Left, styledWordmark...),
			"",
			versionLine,
		)
	} else if width-2 < wordmarkWidth() {
		// Terminal too narrow for the ASCII wordmark: it would overflow the
		// border and get clipped by the terminal itself. Fall back to a
		// plain text lockup instead, same graceful-degradation approach the
		// main frame's header uses below its own narrow-width threshold.
		block = lipgloss.JoinVertical(lipgloss.Center, styles.Wordmark.Render("◆ toolsniff"), "", versionLine)
	} else {
		styledWordmark := make([]string, len(lines))
		for i, l := range lines {
			styledWordmark[i] = styles.Wordmark.Render(l)
		}
		wordmark := lipgloss.JoinVertical(lipgloss.Left, styledWordmark...)
		block = lipgloss.JoinVertical(lipgloss.Center, wordmark, "", versionLine)
	}

	inner := lipgloss.Place(width-2, height-2, lipgloss.Center, lipgloss.Center, block)
	return styles.SplashBorder.Width(width - 2).Height(height - 2).Render(inner)
}

// wordmarkWidth returns the widest line in wordmarkLines, in cells.
func wordmarkWidth() int { return linesWidth(wordmarkLines) }

func linesWidth(lines []string) int {
	max := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > max {
			max = w
		}
	}
	return max
}
