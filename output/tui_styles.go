package output

import (
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/pranvgarg/toolsniff/config"
	"github.com/pranvgarg/toolsniff/model"
)

// This file is the single implementation of DESIGN.md. Every color, weight,
// and padding used anywhere in the TUI is defined here; no component picks a
// color of its own. If a style is missing, add it here rather than reaching
// for lipgloss.Color at a call site.

// tone is one design token expressed across the three color depths a terminal
// may support. DESIGN.md > Colors > "Adaptive downsampling" documents the
// fallbacks: the semantic separation between installed/available/broken has to
// survive on a 16-color terminal, so the ANSI values are chosen by hand rather
// than left to a generic quantizer.
type tone struct {
	ansi    string
	ansi256 string
	hex     string
}

func mono(hex string) tone { return tone{ansi: hex, ansi256: hex, hex: hex} }

// designTokens are the DESIGN.md front-matter values, verbatim.
var designTokens = struct {
	background, surface, surfaceSelected tone
	border, borderFocus                  tone
	foreground, muted                    tone
	accent, accentContrast               tone
	installed, available, broken         tone
	shadowed, history                    tone
	warning, warningSurface, errorTone   tone
}{
	background:      tone{"0", "232", "#0B0D10"},
	surface:         tone{"0", "234", "#12161C"},
	surfaceSelected: tone{"0", "236", "#1E2733"},
	border:          tone{"8", "243", "#566171"},
	borderFocus:     tone{"3", "215", "#FFB454"},
	foreground:      tone{"7", "253", "#E6EAF0"},
	muted:           tone{"8", "246", "#8B95A5"},
	accent:          tone{"3", "215", "#FFB454"},
	accentContrast:  tone{"0", "232", "#0B0D10"},
	installed:       tone{"2", "114", "#5FD68A"},
	available:       tone{"4", "110", "#93A4C0"},
	broken:          tone{"1", "210", "#FF7A7A"},
	shadowed:        tone{"3", "221", "#FFC24B"},
	history:         tone{"5", "141", "#B49BE0"},
	warning:         tone{"3", "221", "#FFC24B"},
	warningSurface:  tone{"0", "236", "#4A3215"},
	errorTone:       tone{"1", "210", "#FF7A7A"},
}

// Palette is the resolved token set for one theme. Fields map 1:1 onto the
// `colors:` block of DESIGN.md.
type Palette struct {
	Background      tone
	Surface         tone
	SurfaceSelected tone
	Border          tone
	BorderFocus     tone
	Foreground      tone
	Muted           tone
	Accent          tone
	AccentContrast  tone
	Installed       tone
	Available       tone
	Broken          tone
	Shadowed        tone
	History         tone
	Warning         tone
	WarningSurface  tone
	Error           tone
}

func designPalette() Palette {
	t := designTokens
	return Palette{
		Background:      t.background,
		Surface:         t.surface,
		SurfaceSelected: t.surfaceSelected,
		Border:          t.border,
		BorderFocus:     t.borderFocus,
		Foreground:      t.foreground,
		Muted:           t.muted,
		Accent:          t.accent,
		AccentContrast:  t.accentContrast,
		Installed:       t.installed,
		Available:       t.available,
		Broken:          t.broken,
		Shadowed:        t.shadowed,
		History:         t.history,
		Warning:         t.warning,
		WarningSurface:  t.warningSurface,
		Error:           t.errorTone,
	}
}

// applyThemeColors overlays a user- or preset-supplied palette. The config
// layer only carries truecolor hexes, so an overridden token drops its
// hand-picked ANSI fallbacks and is downsampled by the renderer instead.
func applyThemeColors(p Palette, c config.ThemeColors) Palette {
	p.Accent = mono(c.Accent)
	p.BorderFocus = mono(c.Accent)
	p.Foreground = mono(c.Text)
	p.Muted = mono(c.Muted)
	p.Border = mono(c.Border)
	p.AccentContrast = mono(c.SelectionForeground)
	p.SurfaceSelected = mono(c.SelectionBackground)
	p.Warning = mono(c.Warning)
	p.Shadowed = mono(c.Warning)
	p.WarningSurface = mono(c.WarningBackground)
	p.Error = mono(c.Error)
	p.Broken = mono(c.Error)
	p.Installed = mono(c.Success)
	p.Available = mono(c.Secondary)
	p.History = mono(c.Secondary)
	return p
}

// paletteFor resolves the design palette for a theme. The built-in "toolsniff"
// theme *is* DESIGN.md, so it is used verbatim; any other preset or user
// override supplies hue, and the contrast guard below supplies the floor.
func paletteFor(theme config.ThemeSettings) Palette {
	p := designPalette()
	if !isDesignTheme(theme) {
		p = applyThemeColors(p, theme.Colors)
	}
	return guardPalette(p)
}

func isDesignTheme(theme config.ThemeSettings) bool {
	if theme.Colors == (config.ThemeColors{}) {
		return true
	}
	builtin, err := config.ThemeSettingsForPreset("toolsniff")
	if err != nil {
		return false
	}
	return theme.Preset == "toolsniff" && theme.Colors == builtin.Colors
}

// guardPalette enforces the DESIGN.md accessibility contract on a palette that
// may have come from user configuration: every text token clears WCAG AA
// (4.5:1) against the ground it is drawn on, and the frame border clears the
// 3:1 non-text threshold of WCAG 1.4.11. A user theme can shift hue; it cannot
// make the UI unreadable.
func guardPalette(p Palette) Palette {
	bg := p.Background.hex
	sel := p.SurfaceSelected.hex

	// Text tokens are read against both the frame ground and the selected-row
	// block, so they are lifted against whichever is more demanding.
	for _, t := range []*tone{
		&p.Foreground, &p.Muted, &p.Accent,
		&p.Installed, &p.Available, &p.Broken, &p.Shadowed, &p.History,
	} {
		t.hex = ensureContrast(t.hex, bg, 4.5)
		t.hex = ensureContrast(t.hex, sel, 4.5)
		if t.ansi == t.ansi256 {
			// An overridden token carries the same hex in all three slots.
			t.ansi, t.ansi256 = t.hex, t.hex
		}
	}
	p.Warning.hex = ensureContrast(p.Warning.hex, bg, 4.5)
	p.Error.hex = ensureContrast(p.Error.hex, bg, 4.5)
	p.AccentContrast.hex = ensureContrast(p.AccentContrast.hex, p.Accent.hex, 4.5)
	p.Border.hex = ensureContrast(p.Border.hex, bg, 3.0)
	return p
}

// profileOnce caches terminal capability detection. Detect honours NO_COLOR,
// CLICOLOR, CLICOLOR_FORCE, COLORTERM and TERM, so the whole palette collapses
// to weight-and-glyph automatically when the user asks for no color.
var (
	profileOnce  sync.Once
	cachedProf   colorprofile.Profile
	cachedResolv lipgloss.CompleteFunc
)

func terminalProfile() (colorprofile.Profile, lipgloss.CompleteFunc) {
	profileOnce.Do(func() {
		cachedProf = colorprofile.Detect(os.Stdout, os.Environ())
		cachedResolv = lipgloss.Complete(cachedProf)
	})
	return cachedProf, cachedResolv
}

func (t tone) color() color.Color {
	_, resolve := terminalProfile()
	return resolve(lipgloss.Color(t.ansi), lipgloss.Color(t.ansi256), lipgloss.Color(t.hex))
}

// Glyphs is the icon vocabulary. Color is never the only signal, so every
// semantic state also owns a glyph — and every glyph has an ASCII form for
// terminals that cannot be trusted with box drawing.
type Glyphs struct {
	Installed string
	Available string
	Broken    string
	Shadowed  string
	History   string
	Changed   string
	Warning   string
	Selection string
	Brand     string

	// Kind markers. These answer "what sort of thing is this" and are always
	// paired with a short text label, so the distinction survives NO_COLOR and
	// ascii mode. DESIGN.md > Components > "Kind marker".
	KindCLI         string
	KindPackage     string
	KindApplication string
	KindExecutable  string
	KindHistory     string

	// Copy affordance shown in front of a runnable action's command line.
	Copy string
}

var unicodeGlyphs = Glyphs{
	Installed: "●", Available: "○", Broken: "✗", Shadowed: "▲",
	History: "·", Changed: "~", Warning: "⚠", Selection: "▎", Brand: "◆",
	KindCLI: "❯", KindPackage: "◇", KindApplication: "▤",
	KindExecutable: "▸", KindHistory: "↺", Copy: "⧉",
}

var asciiGlyphs = Glyphs{
	Installed: "*", Available: "o", Broken: "x", Shadowed: "^",
	History: ".", Changed: "~", Warning: "!", Selection: "|", Brand: "*",
	KindCLI: ">", KindPackage: "#", KindApplication: "@",
	KindExecutable: "=", KindHistory: "~", Copy: "$",
}

// ThemeStyles is the single style vocabulary used by every TUI surface.
// Components consume semantic styles instead of choosing colors themselves.
type ThemeStyles struct {
	// Chrome.
	ActiveTab     lipgloss.Style
	ActiveNewTab  lipgloss.Style
	Tab           lipgloss.Style
	NewTab        lipgloss.Style
	Count         lipgloss.Style
	ActiveCount   lipgloss.Style
	Footer        lipgloss.Style
	FooterKey     lipgloss.Style
	Status        lipgloss.Style
	Warning       lipgloss.Style
	HeaderBorder  lipgloss.Style
	HeaderTitle   lipgloss.Style
	HeaderTagline lipgloss.Style
	HeaderStats   lipgloss.Style
	Wordmark      lipgloss.Style
	Version       lipgloss.Style
	SplashBorder  lipgloss.Style

	// Surfaces.
	Panel       lipgloss.Style
	Modal       lipgloss.Style
	SidebarPane lipgloss.Style
	ContentPane lipgloss.Style
	FramePane   lipgloss.Style

	// Content.
	ColumnHeader lipgloss.Style
	Body         lipgloss.Style
	Meta         lipgloss.Style
	Badge        lipgloss.Style
	EmptyState   lipgloss.Style
	SelectionBar lipgloss.Style
	SelectedRow  lipgloss.Style

	// Semantic rows. DESIGN.md > Colors > "Semantic status".
	RowInstalled lipgloss.Style
	RowAvailable lipgloss.Style
	RowBroken    lipgloss.Style
	RowShadowed  lipgloss.Style
	RowHistory   lipgloss.Style

	// Detail view.
	DetailHeading lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style

	Glyph   Glyphs
	Palette Palette
}

// NewThemeStyles translates the design tokens into Lip Gloss styles. All TUI
// components should receive styles from this factory.
func NewThemeStyles(theme config.ThemeSettings) ThemeStyles {
	p := paletteFor(theme)
	profile, _ := terminalProfile()

	surface := p.Surface.color()
	surfaceSelected := p.SurfaceSelected.color()
	border := p.Border.color()
	borderFocus := p.BorderFocus.color()
	foreground := p.Foreground.color()
	muted := p.Muted.color()
	accent := p.Accent.color()
	accentContrast := p.AccentContrast.color()
	warning := p.Warning.color()
	warningSurface := p.WarningSurface.color()

	glyphs := unicodeGlyphs
	if profile <= colorprofile.Ascii {
		glyphs = asciiGlyphs
	}

	base := lipgloss.NewStyle()

	return ThemeStyles{
		// The active tab pairs the accent with bold+underline so it stays
		// identifiable when color is unavailable.
		ActiveTab:     base.Foreground(accent).Bold(true).Underline(true),
		ActiveNewTab:  base.Foreground(accentContrast).Background(warning).Bold(true),
		Tab:           base.Foreground(muted),
		NewTab:        base.Foreground(warning).Background(warningSurface),
		Count:         base.Foreground(muted),
		ActiveCount:   base.Foreground(accent).Bold(true),
		Footer:        base.Foreground(muted),
		FooterKey:     base.Foreground(foreground).Bold(true),
		Status:        base.Foreground(accent),
		Warning:       base.Foreground(warning).Bold(true),
		HeaderBorder:  base.Foreground(border),
		HeaderTitle:   base.Foreground(accent).Bold(true),
		HeaderTagline: base.Foreground(muted),
		HeaderStats:   base.Foreground(muted),
		Wordmark:      base.Foreground(accent).Bold(true),
		Version:       base.Foreground(muted),
		SplashBorder:  base.Border(lipgloss.RoundedBorder()).BorderForeground(border),

		Panel: base.Border(lipgloss.RoundedBorder()).BorderForeground(border).
			Background(surface).Padding(1, 2),
		Modal: base.Border(lipgloss.RoundedBorder()).BorderForeground(borderFocus).
			Background(surface).Padding(1, 2),
		// Elevation 0: the frame interior. The sidebar's right border is the
		// only divider between the two panes -- see DESIGN.md > Elevation.
		FramePane: base.Border(lipgloss.RoundedBorder()).BorderForeground(border).
			BorderTop(false),
		SidebarPane: base.Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).BorderTop(false).BorderBottom(false).
			BorderLeft(false).BorderForeground(border),
		ContentPane: base.Padding(0, 1),

		ColumnHeader: base.Foreground(muted).Bold(true),
		Body:         base.Foreground(foreground),
		Meta:         base.Foreground(muted),
		Badge:        base.Foreground(accent).Bold(true),
		EmptyState:   base.Foreground(muted).Italic(true),
		SelectionBar: base.Foreground(accent),
		SelectedRow:  base.Background(surfaceSelected).Bold(true),

		RowInstalled: base.Foreground(p.Installed.color()),
		RowAvailable: base.Foreground(p.Available.color()),
		RowBroken:    base.Foreground(p.Broken.color()).Bold(true),
		RowShadowed:  base.Foreground(p.Shadowed.color()),
		RowHistory:   base.Foreground(p.History.color()),

		DetailHeading: base.Foreground(accent).Bold(true),
		DetailLabel:   base.Foreground(muted),
		DetailValue:   base.Foreground(foreground),

		Glyph:   glyphs,
		Palette: p,
	}
}

// rowStyle returns the semantic row style for a status string produced by
// ObservationStatus or rowsForEvents. Anything unrecognized falls back to
// plain body text rather than borrowing another status's meaning.
func (s ThemeStyles) rowStyle(status string) lipgloss.Style {
	switch Status(status) {
	case StatusInstalled, StatusRepaired:
		return s.RowInstalled
	case StatusAvailable:
		return s.RowAvailable
	case StatusBroken:
		return s.RowBroken
	case StatusShadowed:
		return s.RowShadowed
	case StatusHistory:
		return s.RowHistory
	case StatusUpdated, StatusRelocated:
		return s.Body
	}
	switch status {
	case "added":
		return s.RowInstalled
	case "removed":
		return s.RowBroken
	}
	return s.Body
}

// rowGlyph returns the monochrome-safe marker for a status. Color is never the
// only carrier of state -- DESIGN.md > Do's and Don'ts.
func (s ThemeStyles) rowGlyph(status string) string {
	switch Status(status) {
	case StatusInstalled, StatusRepaired:
		return s.Glyph.Installed
	case StatusAvailable:
		return s.Glyph.Available
	case StatusBroken:
		return s.Glyph.Broken
	case StatusShadowed:
		return s.Glyph.Shadowed
	case StatusHistory:
		return s.Glyph.History
	case StatusUpdated, StatusRelocated:
		return s.Glyph.Changed
	}
	switch status {
	case "added":
		return "+"
	case "removed":
		return "-"
	}
	return " "
}

// kindGlyph returns the monochrome-safe marker for an observation kind. It is
// only ever rendered next to kindLabel; neither carries the meaning alone.
func (s ThemeStyles) kindGlyph(kind string) string {
	switch model.ObservationKind(kind) {
	case model.KindCLI:
		return s.Glyph.KindCLI
	case model.KindPackage:
		return s.Glyph.KindPackage
	case model.KindApplication:
		return s.Glyph.KindApplication
	case model.KindExecutable:
		return s.Glyph.KindExecutable
	case model.KindHistory:
		return s.Glyph.KindHistory
	}
	return " "
}

// kindLabel is the compact text half of the kind marker. Change-event rows
// carry no kind, and get nothing rather than a misleading default.
func kindLabel(kind string) string {
	switch model.ObservationKind(kind) {
	case model.KindCLI:
		return "cli"
	case model.KindPackage:
		return "pkg"
	case model.KindApplication:
		return "app"
	case model.KindExecutable:
		return "exe"
	case model.KindHistory:
		return "hist"
	}
	return ""
}

// kindMarker is the glyph+label pair as it appears in the inventory table's
// KIND column.
func (s ThemeStyles) kindMarker(kind string) string {
	label := kindLabel(kind)
	if label == "" {
		return ""
	}
	return s.kindGlyph(kind) + " " + label
}

// --- WCAG contrast helpers -------------------------------------------------
//
// Implemented locally (rather than pulled from a color library) so the guard
// stays a property of the design system and not of a dependency.

func parseHex(hex string) (r, g, b float64, ok bool) {
	hex = strings.TrimSpace(hex)
	if len(hex) != 7 || hex[0] != '#' {
		return 0, 0, 0, false
	}
	value, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64((value >> 16) & 0xFF), float64((value >> 8) & 0xFF), float64(value & 0xFF), true
}

func formatHex(r, g, b float64) string {
	clamp := func(v float64) uint64 {
		switch {
		case v < 0:
			return 0
		case v > 255:
			return 255
		default:
			return uint64(math.Round(v))
		}
	}
	return "#" + strings.ToUpper(pad2(clamp(r))+pad2(clamp(g))+pad2(clamp(b)))
}

func pad2(v uint64) string {
	s := strconv.FormatUint(v, 16)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func channelLuminance(v float64) float64 {
	c := v / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func relativeLuminance(r, g, b float64) float64 {
	return 0.2126*channelLuminance(r) + 0.7152*channelLuminance(g) + 0.0722*channelLuminance(b)
}

// ContrastRatio returns the WCAG 2.x contrast ratio between two #RRGGBB
// colors, or 0 when either value cannot be parsed.
func ContrastRatio(foreground, background string) float64 {
	fr, fg, fb, ok := parseHex(foreground)
	if !ok {
		return 0
	}
	br, bg, bb, ok := parseHex(background)
	if !ok {
		return 0
	}
	l1, l2 := relativeLuminance(fr, fg, fb), relativeLuminance(br, bg, bb)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// ensureContrast walks foreground away from background along the luminance
// axis until it clears target, preserving hue as far as a linear blend toward
// white or black allows. Unparseable input is returned untouched.
func ensureContrast(foreground, background string, target float64) string {
	fr, fg, fb, ok := parseHex(foreground)
	if !ok {
		return foreground
	}
	br, bg, bb, okBG := parseHex(background)
	if !okBG {
		return foreground
	}
	if ContrastRatio(foreground, background) >= target {
		return foreground
	}

	// Move toward whichever pole is further from the background, so a light
	// token on a dark ground gets lighter rather than inverting.
	toward := 255.0
	if relativeLuminance(br, bg, bb) > 0.5 {
		toward = 0.0
	}

	for step := 1; step <= 50; step++ {
		mix := float64(step) / 50
		candidate := formatHex(
			fr+(toward-fr)*mix,
			fg+(toward-fg)*mix,
			fb+(toward-fb)*mix,
		)
		if ContrastRatio(candidate, background) >= target {
			return candidate
		}
	}
	return formatHex(toward, toward, toward)
}
