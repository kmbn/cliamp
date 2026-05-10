package model

import (
	"fmt"
	"strings"

	"cliamp/provider"
	"cliamp/theme"
	"cliamp/ui"
)

func (m Model) renderDeviceOverlay() string {
	lines := []string{
		titleStyle.Render("A U D I O  D E V I C E S"),
		"",
	}

	if m.devicePicker.loading {
		lines = append(lines, loadingLine("Loading devices…"))
		lines = append(lines, "", helpKey("Esc", "Cancel"))
		return m.centerOverlay(strings.Join(lines, "\n"))
	}

	if len(m.devicePicker.devices) == 0 {
		lines = append(lines, dimStyle.Render("  No audio output devices found."))
		lines = append(lines, "", helpKey("Esc", "Close"))
		return m.centerOverlay(strings.Join(lines, "\n"))
	}

	maxVisible := 12
	scroll := scrollStart(m.devicePicker.cursor, maxVisible)
	rendered := 0

	for i := scroll; i < len(m.devicePicker.devices) && i < scroll+maxVisible; i++ {
		d := m.devicePicker.devices[i]
		label := d.Description
		if label == "" {
			label = d.Name
		}
		suffix := ""
		if d.Active {
			suffix = " " + activeToggle.Render("●")
		}
		if i == m.devicePicker.cursor {
			lines = append(lines, playlistSelectedStyle.Render("> "+label)+suffix)
		} else {
			lines = append(lines, dimStyle.Render("  "+label)+suffix)
		}
		rendered++
	}

	lines = padLines(lines, maxVisible, rendered)

	if len(m.devicePicker.devices) > maxVisible {
		lines = append(lines, "", dimStyle.Render(fmt.Sprintf("  %d/%d devices", m.devicePicker.cursor+1, len(m.devicePicker.devices))))
	}

	lines = append(lines, "", helpKey("↓↑", "Scroll ")+helpKey("Enter", "Select ")+helpKey("Esc", "Cancel"))
	return m.centerOverlay(strings.Join(lines, "\n"))
}

func (m Model) renderThemePicker() string {
	lines := []string{
		titleStyle.Render("T H E M E S"),
		"",
	}

	count := len(m.themes) + 1
	maxVisible := m.themePickerVisible()
	scroll := m.themePicker.scroll
	rendered := 0

	for i := scroll; i < count && i < scroll+maxVisible; i++ {
		var name string
		if i == 0 {
			name = theme.DefaultName
		} else {
			name = m.themes[i-1].Name
		}
		lines = append(lines, cursorLine(name, i == m.themePicker.cursor))
		rendered++
	}

	lines = padLines(lines, maxVisible, rendered)
	lines = append(lines, "", dimStyle.Render(fmt.Sprintf("  %d/%d themes", m.themePicker.cursor+1, count)))
	lines = append(lines, "", m.themePickerHelpLine())

	return m.centerOverlay(strings.Join(lines, "\n"))
}

func (m Model) renderInfoOverlay() string {
	track, _ := m.playlist.Current()

	lines := []string{
		titleStyle.Render("T R A C K  I N F O"),
		"",
	}

	field := func(label, value string) {
		if value != "" {
			lines = append(lines, dimStyle.Render("  "+label+": ")+trackStyle.Render(value))
		}
	}

	field("Title", track.Title)
	field("Artist", track.Artist)
	field("Album", track.Album)
	field("Genre", track.Genre)
	if track.Year != 0 {
		field("Year", fmt.Sprintf("%d", track.Year))
	}
	if track.TrackNumber != 0 {
		field("Track", fmt.Sprintf("%d", track.TrackNumber))
	}
	field("Path", track.Path)

	lines = append(lines, "", helpKey("Esc", "Close"))

	return m.centerOverlay(strings.Join(lines, "\n"))
}

func (m Model) renderSearchOverlay() string {
	lines := []string{
		titleStyle.Render("S E A R C H"),
		"",
		playlistSelectedStyle.Render("  / " + m.search.query + "_"),
		"",
	}

	tracks := m.playlist.Tracks()
	maxVisible := 12
	rendered := 0

	if len(m.search.results) == 0 {
		if m.search.query != "" {
			lines = append(lines, dimStyle.Render("  No matches"))
		} else {
			lines = append(lines, dimStyle.Render("  Type to search…"))
		}
		rendered = 1
	} else {
		currentIdx := m.playlist.Index()
		scroll := scrollStart(m.search.cursor, maxVisible)

		for j := scroll; j < scroll+maxVisible && j < len(m.search.results); j++ {
			i := m.search.results[j]
			prefix := "  "
			style := dimStyle

			if i == currentIdx && m.player.IsPlaying() {
				prefix = "▶ "
				style = playlistActiveStyle
			}

			if j == m.search.cursor {
				style = playlistSelectedStyle
			}

			name := tracks[i].DisplayName()
			name = truncate(name, ui.PanelWidth-8)

			line := fmt.Sprintf("%s%d. %s", prefix, i+1, name)
			lines = append(lines, style.Render(line))
			rendered++
		}
	}

	lines = padLines(lines, maxVisible, rendered)
	lines = append(lines, "", dimStyle.Render(fmt.Sprintf("  %d found", len(m.search.results))))
	lines = append(lines, "", helpKey("↓↑", "Scroll ")+helpKey("Enter", "Play ")+helpKey("Tab", "Queue ")+helpKey("Ctrl+K", "Keymap ")+helpKey("Esc", "Close"))

	return m.centerOverlay(strings.Join(lines, "\n"))
}

func (m Model) renderURLInputOverlay() string {
	lines := []string{
		titleStyle.Render("L O A D   U R L"),
		"",
		playlistSelectedStyle.Render("  URL: " + m.urlInput + "_"),
		"",
		helpKey("Enter", "Load") + " " + helpKey("Esc", "Cancel"),
	}
	return m.centerOverlay(strings.Join(lines, "\n"))
}

// stationsHelpLine returns the help footer shown at the bottom of the stations overlay.
func (m Model) stationsHelpLine() string {
	help := helpKey("↓↑", "Scroll ") + helpKey("Enter", "Play ") + helpKey("/", "Search ")
	if _, ok := m.provider.(provider.FavoriteToggler); ok {
		help += helpKey("f", "Fav ")
	}
	return help + helpKey("Ctrl+R", "Refresh ") + helpKey("Esc", "Close")
}

// stationsBudget returns the number of list rows that fit inside the stations overlay,
// accounting for the title, optional search bar, and help footer.
func (m *Model) stationsBudget() int {
	header := []string{titleStyle.Render("S T A T I O N S"), ""}
	if m.provSearch.active {
		header = append(header, playlistSelectedStyle.Render("  / "+m.provSearch.query+"_"), "")
	}
	probe := append(header, "x", "", m.stationsHelpLine())
	return m.measureOverlayVisible(probe, maxPlVisible)
}

// renderStationsOverlay renders the full-screen station browser overlay.
func (m Model) renderStationsOverlay() string {
	budget := m.stationsBudget()

	lines := []string{
		titleStyle.Render("S T A T I O N S"),
		"",
	}

	body := m.renderProviderList(budget)
	lines = append(lines, strings.Split(body, "\n")...)
	lines = append(lines, "", m.stationsHelpLine())

	return m.centerOverlay(strings.Join(lines, "\n"))
}
