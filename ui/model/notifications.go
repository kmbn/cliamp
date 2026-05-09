package model

import (
	"strings"
	"time"

	"cliamp/internal/playback"
	"cliamp/playlist"
	"cliamp/provider"
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

// nowPlaying fires a now-playing notification for the given track if configured.
func (m *Model) nowPlaying(track playlist.Track) {
	reporter := m.findPlaybackReporter(track)
	if reporter == nil {
		return
	}
	canSeek := m.player.Seekable()
	go reporter.ReportNowPlaying(track, m.player.Position(), canSeek)
}

// maybeScrobble fires a playback-complete report for the given track if all
// conditions are met:
//   - a provider claims the track via provider metadata
//   - the track reached at least 50% of its known duration
//
// The call is dispatched in a goroutine so it never blocks the UI. The same
// 50% threshold gates a local history entry so skipped tracks never land in
// "Recently Played".
func (m *Model) maybeScrobble(track playlist.Track, elapsed, duration time.Duration) {
	dur := duration
	if dur <= 0 {
		dur = time.Duration(track.DurationSecs) * time.Second
	}
	pastThreshold := dur > 0 && elapsed >= dur/2

	// Record into local history regardless of provider. Live streams without
	// duration are filtered by pastThreshold. The write is synchronous so
	// successive scrobbles preserve their ordering on disk; the file is small
	// (~30 KB at the 200-entry cap) so the latency is sub-millisecond.
	if pastThreshold && m.historyStore != nil {
		_ = m.historyStore.Record(track, time.Now())
	}

	reporter := m.findPlaybackReporter(track)
	if reporter == nil {
		return
	}
	if duration <= 0 {
		// Unknown duration: use DurationSecs metadata as fallback.
		duration = time.Duration(track.DurationSecs) * time.Second
	}
	if duration <= 0 {
		return // still unknown — skip
	}
	if elapsed < duration/2 {
		return // less than 50% played
	}
	canSeek := m.player.Seekable()
	go reporter.ReportScrobble(track, elapsed, duration, canSeek)
}

// findPlaybackReporter returns the first registered provider that can report
// playback for the given track.
func (m *Model) findPlaybackReporter(track playlist.Track) provider.PlaybackReporter {
	match := func(p playlist.Provider) provider.PlaybackReporter {
		reporter, ok := p.(provider.PlaybackReporter)
		if !ok || !reporter.CanReportPlayback(track) {
			return nil
		}
		return reporter
	}

	if reporter := match(m.provider); reporter != nil {
		return reporter
	}
	for _, pe := range m.providers {
		if pe.Provider == nil {
			continue
		}
		if reporter := match(pe.Provider); reporter != nil {
			return reporter
		}
	}
	return nil
}
