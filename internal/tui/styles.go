package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette.
var (
	colorRosewater = lipgloss.Color("#f5e0dc")
	colorFlamingo  = lipgloss.Color("#f2cdcd")
	colorPink      = lipgloss.Color("#f5c2e7")
	colorMauve     = lipgloss.Color("#cba6f7")
	colorRed       = lipgloss.Color("#f38ba8")
	colorMaroon    = lipgloss.Color("#eba0ac")
	colorPeach     = lipgloss.Color("#fab387")
	colorYellow    = lipgloss.Color("#f9e2af")
	colorGreen     = lipgloss.Color("#a6e3a1")
	colorTeal      = lipgloss.Color("#94e2d5")
	colorSky       = lipgloss.Color("#89dceb")
	colorSapphire  = lipgloss.Color("#74c7ec")
	colorBlue      = lipgloss.Color("#89b4fa")
	colorLavender  = lipgloss.Color("#b4befe")

	colorText     = lipgloss.Color("#cdd6f4")
	colorSubtext1 = lipgloss.Color("#bac2de")
	colorSubtext0 = lipgloss.Color("#a6adc8")
	colorOverlay2 = lipgloss.Color("#9399b2")
	colorOverlay1 = lipgloss.Color("#7f849c")
	colorOverlay0 = lipgloss.Color("#6c7086")
	colorSurface2 = lipgloss.Color("#585b70")
	colorSurface1 = lipgloss.Color("#45475a")
	colorSurface0 = lipgloss.Color("#313244")
	colorBase     = lipgloss.Color("#1e1e2e")
	colorMantle   = lipgloss.Color("#181825")
	colorCrust    = lipgloss.Color("#11111b")
)

// Semantic color aliases.
var (
	colorPrimary   = colorMauve
	colorSecondary = colorBlue
	colorSubtle    = colorOverlay1
	colorDim       = colorSurface2

	colorNeedsInput = colorPeach
	colorWorking    = colorGreen
	colorStarting   = colorBlue
	colorReady      = colorYellow
	colorDone       = colorYellow
	colorExited     = colorOverlay0
	colorUnknown    = colorRed
	colorWarning    = colorYellow
)

// Status text styles.
var (
	styleNeedsInput = lipgloss.NewStyle().Foreground(colorNeedsInput).Bold(true)
	styleWorking    = lipgloss.NewStyle().Foreground(colorWorking)
	styleStarting   = lipgloss.NewStyle().Foreground(colorStarting)
	styleReady      = lipgloss.NewStyle().Foreground(colorReady)
	styleDone       = lipgloss.NewStyle().Foreground(colorDone)
	styleExited     = lipgloss.NewStyle().Foreground(colorExited)
	styleUnknown    = lipgloss.NewStyle().Foreground(colorUnknown)
)

// Layout panes.
var (
	// Outer frame wrapping the whole app.
	styleFrame = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary)

	// Table header row.
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSurface1)

	// Table row (normal).
	styleRow = lipgloss.NewStyle().
			Foreground(colorText)

	// Selected row.
	styleSelected = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	// Dimmed row (done/exited).
	styleDimmed = lipgloss.NewStyle().
			Foreground(colorDim)

	// Help bar at bottom.
	styleHelp = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleHelpKey = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// Warning banner.
	styleWarningBanner = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true).
				Padding(0, 1)

	// Empty state.
	styleEmptyTitle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Bold(true)

	styleEmptyHint = lipgloss.NewStyle().
			Foreground(colorDim)

	// Status dot.
	styleDot = lipgloss.NewStyle()

	// Kill confirmation dialog.
	styleDialog = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPeach).
			Padding(1, 2).
			Foreground(colorText)

	styleDialogTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPeach)

	styleDialogHint = lipgloss.NewStyle().
			Foreground(colorSubtle)

	// Picker match highlight.
	stylePickerMatch = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	// Picker selected row background.
	stylePickerSelected = lipgloss.NewStyle().
				Background(colorSurface1).
				Foreground(colorMauve).
				Bold(true)

	// Picker dialog — wider than the small confirmation dialogs.
	stylePickerDialog = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 3).
				Width(60).
				Foreground(colorText)
)
