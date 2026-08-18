package cli

import "github.com/charmbracelet/lipgloss"

type styles struct {
	title      lipgloss.Style
	item       lipgloss.Style
	sel        lipgloss.Style
	dim        lipgloss.Style
	err        lipgloss.Style
	info       lipgloss.Style
	key        lipgloss.Style
	border     lipgloss.Style
	rating     lipgloss.Style
	subtitle   lipgloss.Style
	progress   lipgloss.Style
	progressBg lipgloss.Style
	sidebar    lipgloss.Style
	sidebarSel lipgloss.Style
	panel      lipgloss.Style
	panelTitle lipgloss.Style
	spinner    lipgloss.Style
	success    lipgloss.Style
	warn       lipgloss.Style
	helpKey    lipgloss.Style
	helpDesc   lipgloss.Style
	badge      lipgloss.Style
	separator  lipgloss.Style
}

var s styles

func initStyles(theme string) {
	if theme == "light" {
		s = lightTheme()
	} else {
		s = darkTheme()
	}
}

// darkTheme: clean neutral terminal palette (no heavy purple).
func darkTheme() styles {
	return styles{
		title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E6EDF3")).Padding(0, 1),
		item:       lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#C9D1D9")),
		sel:        lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0D1117")).Background(lipgloss.Color("#58A6FF")).Bold(true),
		dim:        lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")),
		err:        lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true),
		info:       lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")),
		key:        lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true),
		border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363D")),
		rating:     lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")),
		subtitle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")),
		progress:   lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")),
		progressBg: lipgloss.NewStyle().Foreground(lipgloss.Color("#30363D")),
		sidebar:    lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#C9D1D9")).Background(lipgloss.Color("#161B22")),
		sidebarSel: lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0D1117")).Background(lipgloss.Color("#58A6FF")).Bold(true),
		panel:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#30363D")).Padding(0, 1),
		panelTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E6EDF3")).Padding(0, 1),
		spinner:    lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF")).Bold(true),
		success:    lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")).Bold(true),
		warn:       lipgloss.NewStyle().Foreground(lipgloss.Color("#D29922")).Bold(true),
		helpKey:    lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true),
		helpDesc:   lipgloss.NewStyle().Foreground(lipgloss.Color("#C9D1D9")),
		badge:      lipgloss.NewStyle().Foreground(lipgloss.Color("#0D1117")).Background(lipgloss.Color("#58A6FF")).Padding(0, 1).Bold(true),
		separator:  lipgloss.NewStyle().Foreground(lipgloss.Color("#30363D")),
	}
}

func lightTheme() styles {
	return styles{
		title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1F2328")).Padding(0, 1),
		item:       lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#1F2328")),
		sel:        lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#0969DA")).Bold(true),
		dim:        lipgloss.NewStyle().Foreground(lipgloss.Color("#656D76")),
		err:        lipgloss.NewStyle().Foreground(lipgloss.Color("#CF222E")).Bold(true),
		info:       lipgloss.NewStyle().Foreground(lipgloss.Color("#656D76")),
		key:        lipgloss.NewStyle().Foreground(lipgloss.Color("#0969DA")).Bold(true),
		border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#D0D7DE")),
		rating:     lipgloss.NewStyle().Foreground(lipgloss.Color("#9A6700")),
		subtitle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0969DA")),
		progress:   lipgloss.NewStyle().Foreground(lipgloss.Color("#1A7F37")),
		progressBg: lipgloss.NewStyle().Foreground(lipgloss.Color("#D0D7DE")),
		sidebar:    lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#1F2328")).Background(lipgloss.Color("#F6F8FA")),
		sidebarSel: lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#0969DA")).Bold(true),
		panel:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#D0D7DE")).Padding(0, 1),
		panelTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1F2328")).Padding(0, 1),
		spinner:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0969DA")).Bold(true),
		success:    lipgloss.NewStyle().Foreground(lipgloss.Color("#1A7F37")).Bold(true),
		warn:       lipgloss.NewStyle().Foreground(lipgloss.Color("#9A6700")).Bold(true),
		helpKey:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0969DA")).Bold(true),
		helpDesc:   lipgloss.NewStyle().Foreground(lipgloss.Color("#1F2328")),
		badge:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#0969DA")).Padding(0, 1).Bold(true),
		separator:  lipgloss.NewStyle().Foreground(lipgloss.Color("#D0D7DE")),
	}
}

var camiloDevStyle = lipgloss.NewStyle().
	Bold(true).
	Italic(true).
	Foreground(lipgloss.Color("#79C0FF"))
