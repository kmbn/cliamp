package model

import (
	"regexp"
	"testing"

	"charm.land/lipgloss/v2"

	"cliamp/playlist"
	"cliamp/ui"
)

func withFrameWidth(t *testing.T, width int) {
	t.Helper()
	prevFrameStyle := ui.FrameStyle
	prevPanelWidth := ui.PanelWidth
	ui.FrameStyle = ui.FrameStyle.Width(width)
	ui.PanelWidth = max(0, width-2*ui.PaddingH)
	t.Cleanup(func() {
		ui.FrameStyle = prevFrameStyle
		ui.PanelWidth = prevPanelWidth
	})
}

func TestMainViewShrinksPlaylistForFooterMessages(t *testing.T) {
	if sharedPlayer == nil {
		t.Skip("audio hardware unavailable")
	}
	withFrameWidth(t, 80)

	pl := playlist.New()
	pl.Add(playlist.Track{
		Path:  "http://stream.example.com/station",
		Title: "Radio Station",
	})

	m := Model{
		player:    sharedPlayer,
		playlist:  pl,
		vis:       ui.NewVisualizer(float64(sharedPlayer.SampleRate())),
		width:     80,
		plVisible: 3,
	}
	m.vis.Mode = ui.VisNone
	m.status.Show("Saved", statusTTLDefault)
	m.height = m.mainFrameFixedLines(true) + 1

	if got := m.effectivePlaylistVisible(); got != 1 {
		t.Fatalf("effectivePlaylistVisible() = %d, want 1 with one row left after footer lines", got)
	}
	if got := lipgloss.Height(m.View().Content); got > m.height {
		t.Fatalf("View() height = %d, want <= %d after footer lines shrink playlist", got, m.height)
	}
}

func TestViewConsumesInitialVisualizerRefresh(t *testing.T) {
	if sharedPlayer == nil {
		t.Skip("audio hardware unavailable")
	}
	withFrameWidth(t, 80)

	m := Model{
		player:   sharedPlayer,
		playlist: playlist.New(),
		vis:      ui.NewVisualizer(float64(sharedPlayer.SampleRate())),
		width:    80,
		height:   24,
	}

	if !m.vis.RefreshPending() {
		t.Fatal("refreshPending = false on new visualizer, want initial refresh request")
	}

	_ = m.View()

	if m.vis.RefreshPending() {
		t.Fatal("refreshPending = true after first View(), want refresh consumed")
	}
	if m.vis.Frame() != 1 {
		t.Fatalf("visualizer frame after first View() = %d, want 1", m.vis.Frame())
	}
}

func TestViewKeepsOverlayLayoutUnchanged(t *testing.T) {
	withFrameWidth(t, 80)

	m := Model{
		width:  80,
		height: 22,
		keymap: keymapOverlay{
			visible: true,
		},
	}

	want := m.renderKeymapOverlay()
	if got := lipgloss.Height(want); got > m.height {
		t.Fatalf("renderKeymapOverlay() height = %d, want <= %d for test setup", got, m.height)
	}
	if got := m.View().Content; got != want {
		t.Fatalf("View() changed overlay layout")
	}
}

func TestFullVisualizerViewFitsTerminalWidth(t *testing.T) {
	if sharedPlayer == nil {
		t.Skip("audio hardware unavailable")
	}
	withFrameWidth(t, 80)

	sharedPlayer.Stop()

	m := Model{
		player:   sharedPlayer,
		playlist: playlist.New(),
		vis:      ui.NewVisualizer(float64(sharedPlayer.SampleRate())),
		width:    80,
		height:   24,
		fullVis:  true,
	}
	m.vis.Mode = ui.VisNone

	if got := lipgloss.Width(m.View().Content); got > m.width {
		t.Fatalf("View() width = %d, want <= %d in full visualizer mode", got, m.width)
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

func stripAnsi(str string) string {
	return ansi.ReplaceAllString(str, "")
}
