package ui

import "github.com/charmbracelet/lipgloss"

// Chainlink Blocks Color Palette
// Using high-contrast colors optimized for dark terminal backgrounds
const (
	// White
	ColorWhite = "#FFFFFF"

	// Gray scale
	ColorGray50  = "#FAFBFC"
	ColorGray100 = "#F5F7FA"
	ColorGray200 = "#E4E8ED"
	ColorGray300 = "#D1D6DE"
	ColorGray400 = "#9FA7B2"
	ColorGray500 = "#6C7585"
	ColorGray600 = "#4E5560"
	ColorGray700 = "#3C414C"
	ColorGray800 = "#212732"
	ColorGray900 = "#141921"
	ColorGray950 = "#0E1119"

	// Blue
	ColorBlue50  = "#EFF6FF"
	ColorBlue100 = "#DCEBFF"
	ColorBlue200 = "#C1DBFF"
	ColorBlue300 = "#97C1FF"
	ColorBlue400 = "#639CFF"
	ColorBlue500 = "#2E7BFF"
	ColorBlue600 = "#0D5DFF"
	ColorBlue700 = "#0847F7"
	ColorBlue800 = "#0036C9"
	ColorBlue900 = "#00299A"
	ColorBlue950 = "#001A62"

	// Green
	ColorGreen50  = "#F1FCF5"
	ColorGreen100 = "#DDF8E6"
	ColorGreen200 = "#B9F1CC"
	ColorGreen300 = "#95E5B0"
	ColorGreen400 = "#63D78E"
	ColorGreen500 = "#3CC274"
	ColorGreen600 = "#30A059"
	ColorGreen700 = "#267E46"
	ColorGreen800 = "#1E633A"
	ColorGreen900 = "#195232"
	ColorGreen950 = "#0B2D1B"

	// Red
	ColorRed50  = "#FEF2F2"
	ColorRed100 = "#FEE2E2"
	ColorRed200 = "#FECACA"
	ColorRed300 = "#FCA5A5"
	ColorRed400 = "#F87171"
	ColorRed500 = "#EF4444"
	ColorRed600 = "#DC2626"
	ColorRed700 = "#B91C1C"
	ColorRed800 = "#991B1B"
	ColorRed900 = "#7F1D1D"
	ColorRed950 = "#450A0A"

	// Orange
	ColorOrange50  = "#FEF5EF"
	ColorOrange100 = "#FCE9DA"
	ColorOrange200 = "#FAD3B6"
	ColorOrange300 = "#F6B484"
	ColorOrange400 = "#EF894F"
	ColorOrange500 = "#E86832"
	ColorOrange600 = "#DF4C1C"
	ColorOrange700 = "#B53C19"
	ColorOrange800 = "#913118"
	ColorOrange900 = "#7A2914"
	ColorOrange950 = "#3E130A"

	// Yellow
	ColorYellow50  = "#FFFBEB"
	ColorYellow100 = "#FEF3C7"
	ColorYellow200 = "#FDE68A"
	ColorYellow300 = "#F8D34C"
	ColorYellow400 = "#F9C424"
	ColorYellow500 = "#EAAE06"
	ColorYellow600 = "#CA8A04"
	ColorYellow700 = "#A16207"
	ColorYellow800 = "#854D0E"
	ColorYellow900 = "#713F12"
	ColorYellow950 = "#451A03"

	// Teal
	ColorTeal50  = "#EEFBF9"
	ColorTeal100 = "#DBF5F0"
	ColorTeal200 = "#BFEDE4"
	ColorTeal300 = "#A3E1D5"
	ColorTeal400 = "#80D0C3"
	ColorTeal500 = "#51B9A9"
	ColorTeal600 = "#2F9589"
	ColorTeal700 = "#237872"
	ColorTeal800 = "#1A635E"
	ColorTeal900 = "#124946"
	ColorTeal950 = "#0A2F2F"

	// Purple
	ColorPurple50  = "#F5F2FF"
	ColorPurple100 = "#EDE8FF"
	ColorPurple200 = "#DDD3FF"
	ColorPurple300 = "#C5B2FF"
	ColorPurple400 = "#A787FF"
	ColorPurple500 = "#8657FF"
	ColorPurple600 = "#6838E0"
	ColorPurple700 = "#4B19C1"
	ColorPurple800 = "#3F0DAB"
	ColorPurple900 = "#33068D"
	ColorPurple950 = "#1F005C"
)

// Styles - using Chainlink Blocks palette with high contrast for terminal
var (
	// TitleStyle - for main headers (Blue 500 - bright and visible)
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBlue500))

	// SuccessStyle - for success messages (Green 400 - bright green)
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorGreen400))

	// ErrorStyle - for error messages (Red 400 - high contrast)
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorRed400))

	// WarningStyle - for warnings (Yellow 400 - bright yellow)
	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorYellow400))

	// BoxStyle - for bordered content boxes (Blue 500 border)
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBlue500)).
			Padding(0, 1)

	// DimStyle - for less important/secondary text (Gray 500)
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray500))

	// StepStyle - for step instructions (Blue 400 - lighter, visible)
	StepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBlue400))

	// BoldStyle - plain bold
	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	// CodeStyle - for code/command snippets (Blue 300 - very visible)
	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBlue300))

	// CommandStyle - for CLI commands (Blue 400 - prominent)
	CommandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBlue400))

	// AccentStyle - for highlighted/accent text (Purple 400)
	AccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPurple400))

	// URLStyle - for links (Teal 400 - distinct, underlined)
	URLStyle = lipgloss.NewStyle().
			Underline(true).
			Foreground(lipgloss.Color(ColorTeal400))

	// HighlightStyle - for important highlights (Yellow 300)
	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorYellow300))
)
