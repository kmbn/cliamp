// state.go defines sub-structs that group related fields in the Model,
// making the overall model scannable and maintainable.

package model

import (
	"fmt"
	"strings"
	"time"

	"cliamp/applog"
	"cliamp/player"
)

// searchState holds state for the playlist search overlay.
type searchState struct {
	active  bool
	query   string
	results []int // indices into playlist tracks
	cursor  int
}

// provSearchState holds state for filtering the provider playlist list.
type provSearchState struct {
	active  bool
	query   string
	results []int // indices into providerLists
	cursor  int
}

// seekState holds debounce state for yt-dlp seek-by-restart.
type seekState struct {
	active    bool          // true from first keypress until seek completes
	targetPos time.Duration // absolute target position
	timer     int           // tick countdown for debounce (0 = idle)
	grace     int           // ticks to suppress reconnect after seek completes
	timerFor  time.Duration
	graceFor  time.Duration
}

// themePickerState holds state for the theme picker overlay.
type themePickerState struct {
	visible  bool
	cursor   int
	scroll   int
	savedIdx int // themeIdx before opening picker, for cancel/restore
}

// keymapOverlay holds state for the keybindings overlay.
type keymapOverlay struct {
	visible     bool
	cursor      int
	scroll      int
	savedCursor int
	savedScroll int
	searching   bool
	search      string
	filtered    []int         // indices into entries
	entries     []keymapEntry // core keys + plugin keys, rebuilt on openKeymap
}

// catalogBatchState holds state for lazy-loading catalog entries from a provider.CatalogLoader.
type catalogBatchState struct {
	offset  int  // next offset to fetch
	loading bool // true while a fetch is in flight
	done    bool // true when all stations have been loaded
}

// reconnectState holds state for stream auto-reconnect with exponential backoff.
type reconnectState struct {
	attempts int
	at       time.Time
}

// devicePickerState holds state for the audio device picker overlay.
type devicePickerState struct {
	visible bool
	devices []player.AudioDevice
	cursor  int
	loading bool
}

type saveState struct {
	pendingDownloads int
}

func (s saveState) activityText() string {
	switch s.pendingDownloads {
	case 0:
		return ""
	case 1:
		return "Downloading..."
	default:
		return fmt.Sprintf("Downloading... (%d)", s.pendingDownloads)
	}
}

func (s *saveState) startDownload() {
	s.pendingDownloads++
}

// statusTTL is how long a status line stays visible.
type statusTTL time.Duration

func (t statusTTL) expiresAt(now time.Time) time.Time {
	return now.Add(time.Duration(t))
}

// statusMsg holds a temporary status message shown at the bottom of the UI.
type statusMsg struct {
	text      string
	expiresAt time.Time // zero = no active message
}

func (s statusMsg) Expired(now time.Time) bool {
	return !s.expiresAt.IsZero() && !now.Before(s.expiresAt)
}

func (s *statusMsg) Show(text string, ttl statusTTL) {
	s.ShowAt(time.Now(), text, ttl)
}

func (s *statusMsg) Showf(ttl statusTTL, format string, args ...any) {
	s.Show(fmt.Sprintf(format, args...), ttl)
}

func (s *statusMsg) ShowAt(now time.Time, text string, ttl statusTTL) {
	s.text = text
	s.expiresAt = ttl.expiresAt(now)
}

func (s *statusMsg) Clear() {
	*s = statusMsg{}
}

// logLine is a timestamped log message shown in the footer.
type logLine struct {
	text      string
	expiresAt time.Time
}

const logLineTTL = 6 * time.Second

// tickLogLines drains the applog buffer and expires old entries.
func (m *Model) tickLogLines(now time.Time) {
	for _, e := range applog.Drain() {
		text := strings.TrimRight(e.Text, "\n")
		m.logLines = append(m.logLines, logLine{
			text:      text,
			expiresAt: e.At.Add(logLineTTL),
		})
	}
	// Expire old entries.
	n := 0
	for _, l := range m.logLines {
		if now.Before(l.expiresAt) {
			m.logLines[n] = l
			n++
		}
	}
	m.logLines = m.logLines[:n]
}

// networkStats tracks network throughput for the stream status bar.
type networkStats struct {
	speed     float64 // bytes per second (smoothed)
	lastBytes int64
	sampleFor time.Duration
}

type terminalTitleState struct {
	introActive bool
	introOffset int
	introTick   int
}
