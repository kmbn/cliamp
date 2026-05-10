package model

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"cliamp/internal/playback"
	"cliamp/player"
	"cliamp/playlist"
	"cliamp/provider"
	"cliamp/resolve"
)

// — Message types used by tea.Cmd constructors —

// devicesListedMsg carries the result of listing audio output devices.
type devicesListedMsg struct {
	devices []player.AudioDevice
	err     error
}

// deviceSwitchedMsg signals that an audio device switch attempt completed.
type deviceSwitchedMsg struct {
	name string
	err  error
}

// SetEQPresetMsg is sent by Lua plugins to change the EQ preset by name.
// If Bands is non-nil, the bands are applied and the name becomes a custom label.
type SetEQPresetMsg struct {
	Name  string
	Bands *[10]float64 // nil = use built-in preset bands or keep current
}

// ShowStatusMsg is sent by Lua plugins to display a message in the status bar.
// Duration <= 0 falls back to the default status TTL.
type ShowStatusMsg struct {
	Text     string
	Duration time.Duration
}

type tracksLoadedMsg []playlist.Track

// feedsLoadedMsg carries tracks resolved from remote feed/M3U URLs,
// along with the original source URLs so downstream handlers can identify
// the source (e.g. YouTube Radio) without re-scanning external state.
type feedsLoadedMsg struct {
	tracks   []playlist.Track
	urls     []string // original source URLs that produced these tracks
	autoPlay bool     // whether to start playback automatically
}

// feedTrackResolvedMsg carries episodes resolved from a feed track in the playlist.
type feedTrackResolvedMsg struct {
	tracks []playlist.Track
}

// streamPlayedMsg signals that async stream Play() completed.
type streamPlayedMsg struct{ err error }

// streamPreloadedMsg signals that async stream Preload() completed.
type streamPreloadedMsg struct{}

type attachNotifierMsg struct{ notifier playback.Notifier }

// provAuthDoneMsg signals that interactive provider authentication completed.
type provAuthDoneMsg struct{ err error }

// — Command constructors —

func AttachNotifier(notifier playback.Notifier) tea.Msg {
	return attachNotifierMsg{notifier: notifier}
}

func listDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		devices, err := player.ListAudioDevices()
		return devicesListedMsg{devices: devices, err: err}
	}
}

func switchDeviceCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := player.SwitchAudioDevice(name)
		return deviceSwitchedMsg{name: name, err: err}
	}
}

// authenticateProviderCmd runs the interactive auth flow for a provider.
func authenticateProviderCmd(auth playlist.Authenticator) tea.Cmd {
	return func() tea.Msg {
		return provAuthDoneMsg{err: auth.Authenticate()}
	}
}

func fetchPlaylistsCmd(prov playlist.Provider) tea.Cmd {
	return func() tea.Msg {
		pls, err := prov.Playlists()
		if err != nil {
			return err
		}
		return pls
	}
}

func resolveFeedTrackCmd(feedURL string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := resolve.Remote([]string{feedURL})
		if err != nil {
			return err
		}
		return feedTrackResolvedMsg{tracks: tracks}
	}
}

func resolveRemoteCmd(urls []string, autoPlay bool) tea.Cmd {
	return func() tea.Msg {
		tracks, err := resolve.Remote(urls)
		if err != nil {
			return err
		}
		return feedsLoadedMsg{tracks: tracks, urls: urls, autoPlay: autoPlay}
	}
}

func playStreamCmd(p player.Engine, path string, knownDuration time.Duration) tea.Cmd {
	return func() tea.Msg {
		return streamPlayedMsg{err: p.Play(path, knownDuration)}
	}
}

func preloadStreamCmd(p player.Engine, path string, knownDuration time.Duration) tea.Cmd {
	return func() tea.Msg {
		p.Preload(path, knownDuration) // errors silently ignored
		return streamPreloadedMsg{}
	}
}

func preloadLocalCmd(p player.Engine, path string, knownDuration time.Duration) tea.Cmd {
	return func() tea.Msg {
		p.Preload(path, knownDuration)
		return streamPreloadedMsg{}
	}
}

func fetchTracksCmd(prov playlist.Provider, playlistID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := prov.Tracks(playlistID)
		if err != nil {
			return err
		}
		// Resolve PLS/M3U wrapper URLs to actual stream URLs so the
		// player receives a direct audio stream instead of a playlist file.
		tracks = resolveWrapperURLs(tracks)
		return tracksLoadedMsg(tracks)
	}
}

// resolveWrapperURLs expands any PLS/M3U track paths into the actual stream
// URLs they contain. Non-wrapper tracks are passed through unchanged.
func resolveWrapperURLs(tracks []playlist.Track) []playlist.Track {
	var out []playlist.Track
	for _, t := range tracks {
		if playlist.IsURL(t.Path) && (playlist.IsPLS(t.Path) || playlist.IsM3U(t.Path)) {
			resolved, err := resolve.Remote([]string{t.Path})
			if err == nil && len(resolved) > 0 {
				// Preserve the original title/artist on resolved tracks.
				for i := range resolved {
					if resolved[i].Title == "" || resolved[i].Title == resolved[i].Path {
						resolved[i].Title = t.Title
					}
					if resolved[i].Artist == "" {
						resolved[i].Artist = t.Artist
					}
					if t.Realtime {
						resolved[i].Realtime = true
					}
				}
				out = append(out, resolved...)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// catalogSearchMsg carries the result of a provider.CatalogSearcher.SearchCatalog call.
type catalogSearchMsg struct {
	count int
	err   error
}

func fetchCatalogSearchCmd(s provider.CatalogSearcher, query string) tea.Cmd {
	return func() tea.Msg {
		count, err := s.SearchCatalog(query)
		return catalogSearchMsg{count: count, err: err}
	}
}

// — Catalog batch loading for providers with lazy catalogs —

// catalogBatchSize is the number of catalog entries to fetch per page.
const catalogBatchSize = 100

// catalogBatchMsg carries the result of a provider.CatalogLoader.LoadCatalogPage call.
type catalogBatchMsg struct {
	added int
	err   error
}

func fetchCatalogBatchCmd(loader provider.CatalogLoader, offset, limit int) tea.Cmd {
	return func() tea.Msg {
		added, err := loader.LoadCatalogPage(offset, limit)
		return catalogBatchMsg{added: added, err: err}
	}
}
