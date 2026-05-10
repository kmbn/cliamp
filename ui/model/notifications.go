package model

import (
	"strings"

	"cliamp/internal/playback"
	"cliamp/playlist"
)

// notifyAll sends the current playback state to OS media controls.
func (m *Model) notifyAll() {
	m.notifyPlayback()
}

func (m *Model) attachNotifier(notifier playback.Notifier) {
	m.notifier = notifier
	m.notifyAll()
}

// resolveTrackDisplay returns the display artist and title, applying ICY
// stream title override for radio streams.
func (m *Model) resolveTrackDisplay(track playlist.Track) (artist, title string) {
	artist, title = track.Artist, track.Title
	if m.streamTitle != "" && track.Stream {
		if a, t, ok := strings.Cut(m.streamTitle, " - "); ok {
			artist, title = a, t
		} else {
			title = m.streamTitle
		}
	}
	return
}

func (m *Model) notifyPlayback() {
	if m.notifier == nil {
		return
	}
	status := playback.StatusStopped
	if m.player.IsPlaying() {
		if m.player.IsPaused() {
			status = playback.StatusPaused
		} else {
			status = playback.StatusPlaying
		}
	}
	track, _ := m.playlist.Current()
	artist, title := m.resolveTrackDisplay(track)
	m.notifier.Update(playback.State{
		Status: status,
		Track: playback.Track{
			Title:       title,
			Artist:      artist,
			Album:       track.Album,
			Genre:       track.Genre,
			TrackNumber: track.TrackNumber,
			URL:         track.Path,
			Duration:    m.player.Duration(),
		},
		VolumeDB: m.player.Volume(),
		Position: m.player.Position(),
		Seekable: m.player.Seekable(),
	})
}
