package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"cliamp/applog"
	"cliamp/config"
	"cliamp/external/radio"
	"cliamp/internal/appdir"
	"cliamp/internal/appmeta"
	"cliamp/internal/playback"
	"cliamp/internal/resume"
	"cliamp/ipc"
	"cliamp/luaplugin"
	"cliamp/mediactl"
	"cliamp/player"
	"cliamp/playlist"
	"cliamp/resolve"
	"cliamp/theme"
	"cliamp/ui"
	"cliamp/ui/model"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version string

func run(overrides config.Overrides, positional []string, daemon bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	overrides.Apply(&cfg)

	closeLog, appliedLevel, logErr := initLogging(cfg.LogLevel)
	defer closeLog()
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "logging: %v (continuing without file log)\n", logErr)
		applog.Status("logging: %v", logErr)
	} else {
		applog.Info("cliamp starting (version=%s level=%s)", appmeta.Version(), appliedLevel)
	}

	// Radio is the only provider.
	radioProv := radio.New()
	providers := []model.ProviderEntry{
		{Key: "radio", Name: "Radio", Provider: radioProv},
	}

	resolved, err := resolve.Args(positional)
	if err != nil {
		return err
	}

	pl := playlist.New()
	if len(positional) == 0 {
		pl.Add(
			playlist.Track{Path: "http://radio.cliamp.stream/lofi/stream", Title: "Lofi Stream", Stream: true},
			playlist.Track{Path: "http://radio.cliamp.stream/synthwave/stream", Title: "Synthwave Stream", Stream: true},
			playlist.Track{Path: "http://radio.cliamp.stream/edm/stream", Title: "EDM Stream", Stream: true},
		)
	}
	pl.Add(resolved.Tracks...)

	if cfg.AudioDevice != "" {
		cleanup := player.PrepareAudioDevice(cfg.AudioDevice)
		defer cleanup()
	}

	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		if detected := player.DeviceSampleRate(); detected > 0 {
			sampleRate = detected
		} else {
			sampleRate = 44100
		}
	}

	p, err := player.New(player.Quality{
		SampleRate:      sampleRate,
		BufferMs:        cfg.BufferMs,
		ResampleQuality: cfg.ResampleQuality,
		BitDepth:        cfg.BitDepth,
	})
	if err != nil {
		return fmt.Errorf("player: %w", err)
	}
	defer p.Close()

	cfg.ApplyPlayer(p)
	cfg.ApplyPlaylist(pl)
	ui.SetPadding(cfg.PaddingH, cfg.PaddingV)

	if daemon {
		return runDaemon(p, pl, cfg.AutoPlay)
	}

	themes := theme.LoadAll()

	luaMgr, luaErr := luaplugin.New(cfg.Plugins)
	if luaErr != nil {
		fmt.Fprintf(os.Stderr, "lua plugins: %v\n", luaErr)
	}
	if luaMgr != nil {
		defer luaMgr.Close()
		luaMgr.SetReservedKeys(model.ReservedKeys())
	}

	m := model.New(p, pl, providers, "radio", nil, themes, luaMgr, config.SaveFunc{})

	if luaMgr != nil {
		luaMgr.SetStateProvider(luaplugin.StateProvider{
			PlayerState: func() string {
				if !p.IsPlaying() {
					return "stopped"
				}
				if p.IsPaused() {
					return "paused"
				}
				return "playing"
			},
			Position:      func() float64 { return p.Position().Seconds() },
			Duration:      func() float64 { return p.Duration().Seconds() },
			Volume:        func() float64 { return p.Volume() },
			Speed:         func() float64 { return p.Speed() },
			Mono:          func() bool { return p.Mono() },
			RepeatMode:    func() string { return pl.Repeat().String() },
			Shuffle:       func() bool { return pl.Shuffled() },
			EQBands:       func() [10]float64 { return p.EQBands() },
			TrackTitle:    func() string { t, _ := pl.Current(); return t.Title },
			TrackArtist:   func() string { t, _ := pl.Current(); return t.Artist },
			TrackAlbum:    func() string { t, _ := pl.Current(); return t.Album },
			TrackGenre:    func() string { t, _ := pl.Current(); return t.Genre },
			TrackYear:     func() int { t, _ := pl.Current(); return t.Year },
			TrackNumber:   func() int { t, _ := pl.Current(); return t.TrackNumber },
			TrackPath:     func() string { t, _ := pl.Current(); return t.Path },
			TrackIsStream: func() bool { t, _ := pl.Current(); return t.Stream },
			TrackDuration: func() int { t, _ := pl.Current(); return t.DurationSecs },
			PlaylistCount: func() int { return pl.Len() },
			CurrentIndex:  func() int { return pl.Index() },
		})
	}

	if luaMgr != nil {
		if names := luaMgr.Visualizers(); len(names) > 0 {
			m.RegisterLuaVisualizers(names, luaMgr.RenderVis)
		}
	}

	m.SetSeekStepLarge(cfg.SeekStepLargeDuration())
	m.SetInitialDirectory(cfg.InitialDirectory)
	m.SetPendingURLs(resolved.Pending)
	if len(resolved.Tracks) == 0 && len(resolved.Pending) == 0 && pl.Len() == 0 {
		m.StartInProvider()
	}
	if cfg.EQPreset != "" && cfg.EQPreset != "Custom" {
		m.SetEQPreset(cfg.EQPreset, nil)
	}
	if cfg.Theme != "" {
		m.SetTheme(cfg.Theme)
	}
	if cfg.Visualizer != "" {
		m.SetVisualizer(cfg.Visualizer)
	}
	if cfg.AutoPlay {
		m.SetAutoPlay(true)
	}
	if cfg.Compact {
		m.SetCompact(true)
	}

	if len(positional) > 0 {
		if rs := resume.Load(); rs.Path != "" && rs.PositionSec > 0 {
			m.SetResume(rs.Path, rs.PositionSec)
		}
	}

	prog := tea.NewProgram(m)

	svc, svcErr := wireMediaCtl(prog)
	if svcErr == nil && svc != nil {
		defer svc.Close()
	}

	if luaMgr != nil {
		luaMgr.SetControlProvider(luaplugin.ControlProvider{
			SetVolume:   func(db float64) { p.SetVolume(db) },
			SetSpeed:    func(ratio float64) { p.SetSpeed(ratio) },
			SetEQBand:   func(band int, db float64) { p.SetEQBand(band, db) },
			ToggleMono:  func() { p.ToggleMono() },
			TogglePause: func() { p.TogglePause() },
			Stop:        func() { p.Stop() },
			Seek: func(secs float64) {
				_ = p.Seek(time.Duration(secs * float64(time.Second)))
			},
			SetEQPreset: func(name string, bands *[10]float64) {
				prog.Send(model.SetEQPresetMsg{Name: name, Bands: bands})
			},
			Next: func() { prog.Send(playback.NextMsg{}) },
			Prev: func() { prog.Send(playback.PrevMsg{}) },
		})
		luaMgr.SetUIProvider(luaplugin.UIProvider{
			ShowMessage: func(text string, duration time.Duration) {
				prog.Send(model.ShowStatusMsg{Text: text, Duration: duration})
			},
		})
	}

	ipcSrv, ipcErr := ipc.NewServer(ipc.DefaultSocketPath(), ipc.DispatcherFunc(func(msg any) { prog.Send(msg) }))
	if ipcErr != nil {
		fmt.Fprintf(os.Stderr, "ipc: %v\n", ipcErr)
	} else {
		defer ipcSrv.Close()
		if luaMgr != nil {
			ipcSrv.SetPluginDispatcher(luaMgr)
		}
	}

	finalModel, err := mediactl.Run(prog, svc)
	if err != nil {
		return err
	}

	if fm, ok := finalModel.(model.Model); ok {
		themeName := fm.ThemeName()
		if themeName == theme.DefaultName {
			themeName = ""
		}
		_ = config.Save("theme", fmt.Sprintf("%q", themeName))

		if path, secs, pl := fm.ResumeState(); path != "" && secs > 0 {
			resume.Save(path, secs, pl)
		}
	}

	return nil
}

// initLogging always returns a non-nil close func so the caller can defer
// it unconditionally, plus the applied level as a string for diagnostics.
// Errors come back as the third return value; the close func is a no-op
// and the level string is empty in that case.
func initLogging(levelStr string) (func() error, string, error) {
	noop := func() error { return nil }
	level, err := applog.ParseLevel(levelStr)
	if err != nil {
		return noop, "", err
	}
	dir, err := appdir.Dir()
	if err != nil {
		return noop, "", fmt.Errorf("resolve config dir: %w", err)
	}
	closeFn, err := applog.Init(filepath.Join(dir, "cliamp.log"), level)
	if err != nil {
		return noop, "", err
	}
	return closeFn, level.String(), nil
}

func wireMediaCtl(prog *tea.Program) (*mediactl.Service, error) {
	svc, err := mediactl.New(prog.Send)
	if err != nil || svc == nil {
		return svc, err
	}
	go prog.Send(model.AttachNotifier(svc))
	return svc, nil
}

func ipcSend(req ipc.Request) (ipc.Response, error) {
	resp, err := ipc.Send(ipc.DefaultSocketPath(), req)
	if err != nil {
		return resp, err
	}
	if !resp.OK {
		return resp, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

// ipcSendLong is like ipcSend with a caller-chosen deadline, for plugin
// commands that can legitimately run for minutes (e.g. yt-dlp downloads).
func ipcSendLong(req ipc.Request, deadline time.Duration) (ipc.Response, error) {
	resp, err := ipc.SendWithDeadline(ipc.DefaultSocketPath(), req, deadline)
	if err != nil {
		return resp, err
	}
	if !resp.OK {
		return resp, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

func main() {
	appmeta.SetVersion(version)
	app := buildApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
