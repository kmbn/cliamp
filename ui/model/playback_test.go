package model

import (
	"testing"
	"time"

	"cliamp/playlist"
	"cliamp/ui"
)

type playbackFakeEngine struct {
	playing   bool
	playCalls []string
}

func (f *playbackFakeEngine) Play(path string, _ time.Duration) error {
	f.playing = true
	f.playCalls = append(f.playCalls, path)
	return nil
}
func (f *playbackFakeEngine) PlayYTDL(string, time.Duration) error    { return nil }
func (f *playbackFakeEngine) Preload(string, time.Duration) error     { return nil }
func (f *playbackFakeEngine) PreloadYTDL(string, time.Duration) error { return nil }
func (f *playbackFakeEngine) ClearPreload()                           {}
func (f *playbackFakeEngine) Stop()                                   { f.playing = false }
func (f *playbackFakeEngine) Close()                                  {}
func (f *playbackFakeEngine) TogglePause()                            {}
func (f *playbackFakeEngine) Seek(time.Duration) error                { return nil }
func (f *playbackFakeEngine) SeekYTDL(time.Duration) error            { return nil }
func (f *playbackFakeEngine) CancelSeekYTDL()                         {}
func (f *playbackFakeEngine) IsPlaying() bool                         { return f.playing }
func (f *playbackFakeEngine) IsPaused() bool                          { return false }
func (f *playbackFakeEngine) Drained() bool                           { return false }
func (f *playbackFakeEngine) HasPreload() bool                        { return false }
func (f *playbackFakeEngine) Seekable() bool                          { return false }
func (f *playbackFakeEngine) IsStreamSeek() bool                      { return false }
func (f *playbackFakeEngine) IsYTDLSeek() bool                        { return false }
func (f *playbackFakeEngine) GaplessAdvanced() bool                   { return false }
func (f *playbackFakeEngine) Position() time.Duration                 { return 0 }
func (f *playbackFakeEngine) Duration() time.Duration                 { return 0 }
func (f *playbackFakeEngine) PositionAndDuration() (time.Duration, time.Duration) {
	return 0, 0
}
func (f *playbackFakeEngine) SetVolume(float64)                      {}
func (f *playbackFakeEngine) Volume() float64                        { return 0 }
func (f *playbackFakeEngine) SetSpeed(float64)                       {}
func (f *playbackFakeEngine) Speed() float64                         { return 1 }
func (f *playbackFakeEngine) ToggleMono()                            {}
func (f *playbackFakeEngine) Mono() bool                             { return false }
func (f *playbackFakeEngine) SetEQBand(int, float64)                 {}
func (f *playbackFakeEngine) EQBands() [10]float64                   { return [10]float64{} }
func (f *playbackFakeEngine) StreamErr() error                       { return nil }
func (f *playbackFakeEngine) StreamTitle() string                    { return "" }
func (f *playbackFakeEngine) StreamBytes() (downloaded, total int64) { return 0, 0 }
func (f *playbackFakeEngine) SamplesInto([]float64) int              { return 0 }
func (f *playbackFakeEngine) SampleRate() int                        { return 44100 }

func TestTogglePlayPauseStartsPlayback(t *testing.T) {
	player := &playbackFakeEngine{}
	p := playlist.New()
	p.Replace([]playlist.Track{
		{Title: "Track", Path: "track.mp3", DurationSecs: 180},
	})
	p.SetIndex(0)

	m := Model{
		player:   player,
		playlist: p,
		vis:      ui.NewVisualizer(float64(player.SampleRate())),
	}

	if cmd := m.togglePlayPause(); cmd != nil {
		_ = cmd()
	}

	if len(player.playCalls) != 1 || player.playCalls[0] != "track.mp3" {
		t.Fatalf("playCalls = %v, want [track.mp3]", player.playCalls)
	}
}

func TestPlayCurrentTrackPlaysCurrentPosition(t *testing.T) {
	player := &playbackFakeEngine{}
	p := playlist.New()
	p.Replace([]playlist.Track{
		{Title: "Radio A", Path: "http://stream.example.com/a", Stream: true},
		{Title: "Radio B", Path: "http://stream.example.com/b", Stream: true},
	})
	p.SetIndex(1)

	m := Model{
		player:   player,
		playlist: p,
		vis:      ui.NewVisualizer(float64(player.SampleRate())),
	}

	cmd := m.playCurrentTrack()
	if cmd == nil {
		t.Fatal("playCurrentTrack() = nil, want command")
	}
	if m.plCursor != 1 {
		t.Fatalf("plCursor = %d, want 1", m.plCursor)
	}
}

func TestPlayCurrentTrackEmptyPlaylistReturnsNil(t *testing.T) {
	player := &playbackFakeEngine{playing: true}
	p := playlist.New()

	m := Model{
		player:   player,
		playlist: p,
		vis:      ui.NewVisualizer(float64(player.SampleRate())),
	}

	if cmd := m.playCurrentTrack(); cmd != nil {
		t.Fatalf("playCurrentTrack() on empty = %v, want nil", cmd)
	}
}
