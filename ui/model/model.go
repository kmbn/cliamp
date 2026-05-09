// Package ui implements the Bubbletea TUI for the CLIAMP terminal music player.
package model

import (
	"time"

	"cliamp/history"
	"cliamp/internal/playback"
	"cliamp/luaplugin"
	"cliamp/player"
	"cliamp/playlist"
	"cliamp/theme"
	"cliamp/ui"
)

// ConfigSaver persists individual config key-value pairs.
// Satisfied by config.SaveFunc (the default) or a test stub.
type ConfigSaver interface {
	Save(key, value string) error
}

type focusArea int

const (
	focusPlaylist focusArea = iota
	focusEQ
	focusProvPill
	focusSearch
	focusProvider
)

func (f focusArea) label() string {
	switch f {
	case focusPlaylist:
		return "Playlist"
	case focusEQ:
		return "Equalizer"
	case focusProvPill:
		return "Source"
	case focusProvider:
		return "Provider"
	case focusSearch:
		return "Search"
	default:
		return ""
	}
}

type topLevelScreen int

const (
	screenMain topLevelScreen = iota
	screenKeymap
	screenThemePicker
	screenDevicePicker
	screenInfo
	screenSearch
	screenURLInput
	screenFullVisualizer
)

func (s topLevelScreen) hidesVisualizer() bool {
	return s != screenMain && s != screenFullVisualizer
}

// maxPlVisible caps the playlist at a readable height even on tall terminals.
// maxPlExpandVisible is the higher cap used when the user expands with 'x'.
const (
	maxPlVisible       = 12
	maxPlExpandVisible = 24
)

// ProviderEntry pairs a display name with a key and provider implementation.
type ProviderEntry struct {
	Key      string            // config key: "radio", "navidrome", "spotify"
	Name     string            // display name: "Radio", "Navidrome", "Spotify"
	Provider playlist.Provider // nil if not configured
}

// statusTTL* constants define how long a status message is shown.
const (
	statusTTLShort   statusTTL = statusTTL(2 * time.Second)         // brief confirmations
	statusTTLDefault statusTTL = statusTTL(3 * time.Second)         // standard status messages
	statusTTLMedium  statusTTL = statusTTL(4 * time.Second)         // messages needing extra visibility
	statusTTLBatch   statusTTL = statusTTL(4500 * time.Millisecond) // batch operation feedback
	statusTTLLong    statusTTL = statusTTL(6 * time.Second)         // loading indicators
)

// Model is the Bubbletea model for the CLIAMP TUI.
type Model struct {
	// Core playback
	player        player.Engine
	playlist      *playlist.Playlist
	configSaver   ConfigSaver
	vis           *ui.Visualizer
	seekStepLarge time.Duration

	// UI navigation
	focus           focusArea
	prevFocus       focusArea // focus to restore on cancel (search)
	eqCursor        int       // selected EQ band (0-9)
	plCursor        int       // selected playlist item
	plScroll        int       // scroll offset for playlist view
	plVisible       int       // desired max visible playlist lines
	titleOff        int       // scroll offset for long track titles
	titleLastScroll time.Time // last time the title scrolled
	err             error
	quitting        bool
	width           int
	height          int

	// Provider state
	provider      playlist.Provider
	providerLists []playlist.PlaylistInfo
	provCursor    int
	provScroll    int
	provLoading   bool
	provSignIn    bool            // true when provider needs interactive sign-in
	providers     []ProviderEntry // all available providers
	provPillIdx   int             // selected pill index
	eqPresetIdx   int             // -1 = custom, 0+ = index into eqPresets
	eqCustomLabel string          // non-empty = plugin-defined preset label (shown instead of "Custom")

	// Overlay / feature state (see state.go for struct definitions)
	search       searchState
	provSearch   provSearchState
	seek         seekState
	themePicker  themePickerState
	keymap       keymapOverlay
	catalogBatch catalogBatchState
	reconnect    reconnectState
	save         saveState
	status       statusMsg
	logLines     []logLine
	network      networkStats
	speedSaveAfter time.Duration
	termTitle    terminalTitleState

	// URL input mode (load playlist/stream URL at runtime)
	urlInputting bool
	urlInput     string

	// Async feed/M3U URL resolution
	pendingURLs []string
	feedLoading bool

	// Async stream buffering (true while HTTP connect is in progress)
	buffering   bool
	bufferingAt time.Time // when buffering started, for elapsed display

	// resume holds the path and position to seek to when the matching track
	// starts playing. Cleared after the seek is performed.
	resume struct {
		path string
		secs int
	}

	loadedPlaylist string // name of the currently loaded local playlist (for resume)
	// from a non-local provider (Spotify, Navidrome, …). Used to highlight that
	// row in the provider browser. Empty when no provider playlist is active.
	activeProviderPlaylistID string

	// exitResume holds the playback state captured just before player.Close()
	// so ResumeState() can read it after the player is shut down.
	exitResume struct {
		path     string
		secs     int
		playlist string
	}

	// preloading is true while a preloadStreamCmd goroutine is in-flight.
	preloading bool

	// Live stream title from ICY metadata (e.g., "Artist - Song")
	streamTitle string

	notifier playback.Notifier

	// Lua plugin manager (nil if no plugins loaded)
	luaMgr *luaplugin.Manager

	// History recorder (nil if config dir unavailable; safe to call when nil)
	historyStore *history.Store

	// initialDir is the starting path for the file browser ('o' key).
	initialDir string

	// Theme state: -1 = Default (ANSI), 0+ = index into themes
	themes   []theme.Theme
	themeIdx int

	// Track info overlay (metadata details)
	showInfo bool

	// Audio device picker overlay
	devicePicker devicePickerState

	// Full-screen visualizer mode (Shift+V)
	fullVis bool

	autoPlay       bool // start playing immediately on launch
	compact        bool // compact mode: cap frame width at 80 columns
	heightExpanded bool // tracks whether manual 'x' expansion is active

	// Cached per-tick to avoid repeated speaker.Lock() calls in View().
	cachedPos  time.Duration
	cachedDur  time.Duration
	lastTickAt time.Time // wall time of previous tickMsg; used for tick delta

}

func (m Model) activeScreen() topLevelScreen {
	switch {
	case m.keymap.visible:
		return screenKeymap
	case m.themePicker.visible:
		return screenThemePicker
	case m.devicePicker.visible:
		return screenDevicePicker
	case m.showInfo:
		return screenInfo
	case m.search.active:
		return screenSearch
	case m.urlInputting:
		return screenURLInput
	case m.fullVis:
		return screenFullVisualizer
	default:
		return screenMain
	}
}

func (m Model) isOverlayActive() bool {
	return m.activeScreen().hidesVisualizer()
}

func (m Model) isPlaying() bool {
	return m.player != nil && m.player.IsPlaying()
}

func (m Model) isPaused() bool {
	return m.player != nil && m.player.IsPaused()
}
