package model

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"cliamp/playlist"
)

// nextTrack advances to the next playlist track and starts playing it.
// Unplayable tracks are skipped automatically.
func (m *Model) nextTrack() tea.Cmd {
	track, ok := m.playlist.Next()
	if !ok {
		m.player.Stop()
		return nil
	}
	m.plCursor = m.playlist.Index()
	m.adjustScroll()
	return m.playTrack(track)
}

// prevTrack goes to the previous track, or restarts if >3s into the current one.
// Unplayable tracks are skipped automatically.
func (m *Model) prevTrack() tea.Cmd {
	if m.player.Position() > 3*time.Second {
		if m.player.Seekable() {
			// Seekable media rewinds in place; non-seekable streams must be restarted.
			m.player.Seek(-m.player.Position())
			return nil
		}
		track, idx := m.playlist.Current()
		if idx >= 0 {
			return m.playTrack(track)
		}
		return nil
	}
	track, ok := m.playlist.Prev()
	if !ok {
		return nil
	}
	m.plCursor = m.playlist.Index()
	m.adjustScroll()
	return m.playTrack(track)
}

// playCurrentTrack starts playing the currently selected track.
func (m *Model) playCurrentTrack() tea.Cmd {
	m.titleOff = 0
	track, idx := m.playlist.Current()
	if idx < 0 {
		return nil
	}
	m.plCursor = idx
	m.adjustScroll()
	return m.playTrack(track)
}

// removeSelectedFromPlaylist is a no-op in radio mode.
func (m *Model) removeSelectedFromPlaylist() {}

// playTrack plays a track, using async HTTP for streams and sync I/O for local files.
// yt-dlp URLs are streamed via a piped yt-dlp | ffmpeg chain for instant playback.
func (m *Model) playTrack(track playlist.Track) tea.Cmd {
	if track.Feed || playlist.IsFeed(track.Path) {
		m.feedLoading = true
		m.status.Show("Loading feed...", statusTTLLong)
		return resolveFeedTrackCmd(track.Path)
	}

	m.reconnect.attempts = 0
	m.reconnect.at = time.Time{}
	m.streamTitle = ""
	dur := time.Duration(track.DurationSecs) * time.Second
	if track.Stream {
		m.buffering = true
		m.bufferingAt = time.Now()
		m.err = nil
		return playStreamCmd(m.player, track.Path, dur)
	}
	if err := m.player.Play(track.Path, dur); err != nil {
		// Provider session went stale (e.g. Spotify auth expired and
		// silent reconnect failed). Surface the standard sign-in
		// overlay rather than the raw stream error.
		if errors.Is(err, playlist.ErrNeedsAuth) {
			m.provSignIn = true
			m.err = nil
		} else {
			m.err = err
		}
	} else {
		m.err = nil
	}

	return m.preloadNext()
}

// togglePlayPause starts playback if stopped, or toggles pause if playing.
// For live streams, unpausing reconnects to get current audio instead of
// playing stale data sitting in OS/decoder buffers from before the pause.
func (m *Model) togglePlayPause() tea.Cmd {
	if m.buffering {
		return nil
	}
	if !m.player.IsPlaying() {
		return m.playCurrentTrack()
	}
	if m.player.IsPaused() {
		track, idx := m.playlist.Current()
		if shouldReconnectOnUnpause(track, idx) {
			m.player.Stop()
			return m.playTrack(track)
		}
	}
	m.player.TogglePause()
	return nil
}

// shouldReconnectOnUnpause reports whether unpausing should reconnect and
// restart instead of resuming buffered audio.
func shouldReconnectOnUnpause(track playlist.Track, idx int) bool {
	return idx >= 0 && track.IsLive()
}
