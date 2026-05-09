package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"cliamp/internal/fileutil"
	"cliamp/playlist"
	"cliamp/provider"
	"cliamp/ui"
)

// quit shuts down the player and signals the TUI to exit.
func (m *Model) quit() tea.Cmd {
	// Only save resume for seekable tracks:
	// - local files (not stream)
	// - HTTP streams with known duration (podcast MP3s, seek-by-reconnect)
	// Exclude YTDL (position unreliable) and real-time live streams.
	if track, _ := m.playlist.Current(); track.Path != "" &&
		!playlist.IsYTDL(track.Path) && !track.IsLive() &&
		m.player.IsPlaying() {
		if secs := int(m.player.Position().Seconds()); secs > 0 {
			m.exitResume.path = track.Path
			m.exitResume.secs = secs
			m.exitResume.playlist = m.loadedPlaylist
		}
	}

	m.flushPendingSpeedSave()
	m.player.Close()
	m.quitting = true
	return tea.Quit
}

// scrobbleCurrent fires a scrobble for the currently playing track if applicable.
func (m *Model) scrobbleCurrent() {
	if track, idx := m.playlist.Current(); idx >= 0 {
		m.maybeScrobble(track, m.player.Position(), m.player.Duration())
	}
}

func (m *Model) providerScrollStep() int {
	return max(1, m.effectivePlaylistVisible())
}

func (m *Model) providerMaybeAdjustScroll() {
	visible := m.providerScrollStep()
	total := len(m.providerLists)
	if total == 0 {
		m.provScroll = 0
		return
	}

	if m.provCursor < m.provScroll {
		m.provScroll = m.provCursor
	}

	// Sectioned providers (e.g. radio) render extra header rows, so
	// cursor visibility must be computed in rendered rows, not item count.
	if sl, ok := m.provider.(provider.SectionedList); ok {
		if m.provScroll >= total {
			m.provScroll = max(0, total-1)
		}

		// Only push down when needed to keep the cursor visible.
		// Do not "pull up" aggressively, which can make paging feel jumpy
		// and keep the cursor stuck near the bottom of the viewport.
		for m.provScroll < total && m.providerRowsFromScroll(sl, m.provScroll, m.provCursor) > visible {
			m.provScroll++
		}
		return
	}

	// Non-sectioned providers: regular item-count based scrolling.
	if m.provCursor >= m.provScroll+visible {
		m.provScroll = m.provCursor - visible + 1
	}
	if m.provScroll+visible > total {
		m.provScroll = max(0, total-visible)
	}
}

func (m *Model) providerRowsFromScroll(sl provider.SectionedList, scroll, cursor int) int {
	total := len(m.providerLists)
	if total == 0 || cursor < scroll || scroll < 0 || cursor >= total {
		return 0
	}

	rows := 0
	prevPrefix := ""
	if scroll > 0 {
		prevPrefix = sl.IDPrefix(m.providerLists[scroll-1].ID)
	}

	for i := scroll; i <= cursor && i < total; i++ {
		pfx := sl.IDPrefix(m.providerLists[i].ID)
		if pfx != prevPrefix {
			rows++ // section header row
		}
		rows++ // item row
		prevPrefix = pfx
	}
	return rows
}

func (m *Model) providerMoveUp() {
	if m.provCursor > 0 {
		m.provCursor--
	} else if len(m.providerLists) > 0 {
		m.provCursor = len(m.providerLists) - 1
	}
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerMoveDown() {
	if m.provCursor < len(m.providerLists)-1 {
		m.provCursor++
	} else if len(m.providerLists) > 0 {
		m.provCursor = 0
	}
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerPageUp() {
	step := m.providerScrollStep()
	if m.provCursor > 0 {
		m.provCursor -= min(m.provCursor, step)
	}
	// Top-anchor behavior: place cursor at top of viewport when paging up.
	m.provScroll = m.provCursor
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerPageDown() {
	step := m.providerScrollStep()
	if m.provCursor < len(m.providerLists)-1 {
		m.provCursor = min(len(m.providerLists)-1, m.provCursor+step)
	}
	// Bottom-anchor behavior: bias viewport so cursor lands near bottom when paging down.
	m.provScroll = max(0, m.provCursor-step+1)
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerToTop() {
	m.provCursor = 0
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerToBottom() {
	if len(m.providerLists) > 0 {
		m.provCursor = len(m.providerLists) - 1
	}
	m.providerMaybeAdjustScroll()
}

// handleKey processes a single key press and returns an optional command.
func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.keymap.visible {
		return m.handleKeymapKey(msg)
	}

	// Audio device picker overlay
	if m.devicePicker.visible {
		return m.handleDeviceKey(msg)
	}

	// Theme picker overlay — interactive navigation
	if m.themePicker.visible {
		return m.handleThemeKey(msg)
	}

	// Track info overlay
	if m.showInfo {
		switch msg.String() {
		case "ctrl+c":
			return m.quit()
		case "esc", "i":
			m.showInfo = false
		}
		return nil
	}

	if m.urlInputting {
		return m.handleURLInputKey(msg)
	}

	if m.search.active {
		return m.handleSearchKey(msg)
	}

	if m.provSearch.active {
		return m.handleProvSearchKey(msg)
	}

	if m.focus == focusProvider {
		switch msg.String() {
		case "q", "ctrl+c":
			return m.quit()
		case "up", "k":
			m.providerMoveUp()
		case "space":
			return m.togglePlayPause()
		case "down", "j":
			m.providerMoveDown()
			// Auto-load next catalog page when scrolling near the bottom.
			return m.maybeLoadCatalogBatch()
		case "enter":
			if m.provSignIn {
				if auth, ok := m.provider.(playlist.Authenticator); ok {
					m.provSignIn = false
					m.provLoading = true
					return authenticateProviderCmd(auth)
				}
			}
			if len(m.providerLists) > 0 && !m.provLoading {
				m.provLoading = true
				m.activeProviderPlaylistID = m.providerLists[m.provCursor].ID
				return fetchTracksCmd(m.provider, m.providerLists[m.provCursor].ID)
			}
		case "tab":
			m.focus = focusEQ
		case "esc", "backspace", "b":
			// If viewing catalog search results, clear them first.
			if cs, ok := m.provider.(provider.CatalogSearcher); ok && cs.IsSearching() {
				m.restoreCatalog(cs)
				return nil
			}
			if m.playlist.Len() > 0 {
				m.focus = focusPlaylist
			}
		case "/":
			m.provSearch.active = true
			m.provSearch.query = ""
			m.provSearch.results = nil
			m.provSearch.cursor = 0
		case "ctrl+r":
			if m.provider != nil && !m.provLoading {
				if r, ok := m.provider.(playlist.Refresher); ok {
					r.Refresh()
				}
				m.providerLists = nil
				m.provLoading = true
				m.activeProviderPlaylistID = ""
				m.status.Showf(statusTTLShort, "Refreshing %s…", m.provider.Name())
				return fetchPlaylistsCmd(m.provider)
			}
		case "f":
			return m.toggleProviderFavorite()
		case "pgup", "ctrl+u":
			m.providerPageUp()
		case "pgdown", "ctrl+d":
			m.providerPageDown()
			return m.maybeLoadCatalogBatch()
		case "g", "home":
			m.providerToTop()
		case "G", "end":
			m.providerToBottom()
			return m.maybeLoadCatalogBatch()
		case "J":
			return m.switchToProvider("jellyfin")
		case "E":
			return m.switchToProvider("emby")
		case "S":
			return m.switchToProvider("spotify")
		case "C":
			return m.switchToProvider("soundcloud")
		case "L":
			return m.switchToProvider("local")
		case "R":
			return m.switchToProvider("radio")
		case "ctrl+x":
			m.toggleExpandedView()
		case "ctrl+f":
			m.openProviderSearch()
		}
		return nil
	}

	if m.focus == focusProvPill {
		switch msg.String() {
		case "q", "ctrl+c":
			return m.quit()
		case "left", "h":
			if m.provPillIdx > 0 {
				m.provPillIdx--
			}
		case "right", "l":
			if m.provPillIdx < len(m.providers)-1 {
				m.provPillIdx++
			}
		case "enter":
			return m.switchProvider(m.provPillIdx)
		case "tab":
			m.focus = focusPlaylist
		case "esc", "backspace":
			m.focus = focusEQ
		case "space":
			return m.togglePlayPause()
		}
		return nil
	}

	// Main key dispatch.
	switch msg.String() {
	case "q", "ctrl+c":
		return m.quit()
	case "esc", "backspace", "b":
		if m.fullVis {
			m.fullVis = false
			m.vis.Rows = ui.DefaultVisRows
			m.restorePanelWidth()
		} else if m.focus == focusPlaylist {
			// Keep current expanded/collapsed height mode when switching focus.
			m.focus = focusProvider
		}

	case "space":
		cmd := m.togglePlayPause()
		m.notifyPlayback()
		return cmd

	case "s":
		m.player.Stop()
		m.notifyPlayback()

	case ">", ".":
		m.scrobbleCurrent()
		cmd := m.nextTrack()
		m.notifyPlayback()
		return cmd

	case "<", ",":
		m.scrobbleCurrent()
		cmd := m.prevTrack()
		m.notifyPlayback()
		return cmd

	case "left":
		if m.focus == focusEQ {
			if m.eqCursor > 0 {
				m.eqCursor--
			}
		} else {
			return m.doSeek(-5 * time.Second)
		}

	case "shift+left":
		return m.doSeek(-m.seekStepLarge)

	case "right":
		if m.focus == focusEQ {
			if m.eqCursor < eqBandCount-1 {
				m.eqCursor++
			}
		} else {
			return m.doSeek(5 * time.Second)
		}

	case "shift+right":
		return m.doSeek(m.seekStepLarge)

	case "up", "k":
		if m.focus == focusEQ {
			bands := m.player.EQBands()
			m.player.SetEQBand(m.eqCursor, bands[m.eqCursor]+1)
			m.eqPresetIdx = -1 // manual tweak → custom
			m.eqCustomLabel = ""
			m.saveEQ()
		} else {
			if m.plCursor > 0 {
				m.plCursor--
				m.adjustScroll()
			} else if m.playlist.Len() > 0 {
				m.plCursor = m.playlist.Len() - 1
				m.adjustScroll()
			}
		}

	case "down", "j":
		if m.focus == focusEQ {
			bands := m.player.EQBands()
			m.player.SetEQBand(m.eqCursor, bands[m.eqCursor]-1)
			m.eqPresetIdx = -1 // manual tweak → custom
			m.eqCustomLabel = ""
			m.saveEQ()
		} else {
			if m.plCursor < m.playlist.Len()-1 {
				m.plCursor++
				m.adjustScroll()
			} else if m.playlist.Len() > 0 {
				m.plCursor = 0
				m.adjustScroll()
			}
		}

	case "pgup", "ctrl+u":
		if m.focus == focusPlaylist && m.plCursor > 0 {
			visible := max(1, m.effectivePlaylistVisible())
			m.plCursor -= min(m.plCursor, visible)
			m.adjustScroll()
		}

	case "pgdown", "ctrl+d":
		if m.focus == focusPlaylist && m.plCursor < m.playlist.Len()-1 {
			visible := max(1, m.effectivePlaylistVisible())
			m.plCursor = min(m.playlist.Len()-1, m.plCursor+visible)
			m.adjustScroll()
		}

	case "g", "home":
		if m.focus == focusPlaylist && m.plCursor != 0 {
			m.plCursor = 0
			m.adjustScroll()
		}

	case "G", "end":
		if m.focus == focusPlaylist && m.playlist.Len() > 0 && m.plCursor != m.playlist.Len()-1 {
			m.plCursor = m.playlist.Len() - 1
			m.adjustScroll()
		}

	case "enter":
		if m.focus == focusPlaylist {
			// No-op only if this exact track is still buffering.
			if m.buffering && m.plCursor == m.playlist.Index() {
				break
			}
			m.scrobbleCurrent()
			m.playlist.SetIndex(m.plCursor)
			cmd := m.playCurrentTrack()
			m.notifyPlayback()
			return cmd
		}

	case "+", "=":
		m.player.SetVolume(m.player.Volume() + 1)
		m.notifyPlayback()

	case "-":
		m.player.SetVolume(m.player.Volume() - 1)
		m.notifyPlayback()

	case "tab":
		switch m.focus {
		case focusPlaylist:
			m.focus = focusEQ
		case focusEQ:
			if len(m.providers) > 1 {
				m.focus = focusProvPill
			} else {
				m.focus = focusPlaylist
			}
		case focusProvPill:
			m.focus = focusPlaylist
		default:
			m.focus = focusPlaylist
		}

	case "h":
		if m.focus == focusEQ && m.eqCursor > 0 {
			m.eqCursor--
		}

	case "l":
		if m.focus == focusEQ && m.eqCursor < eqBandCount-1 {
			m.eqCursor++
		}

	case "e":
		m.eqPresetIdx++
		if m.eqPresetIdx >= len(eqPresets) {
			m.eqPresetIdx = 0
		}
		m.applyEQPreset()
		m.saveEQ()

	case "ctrl+s":
		return m.saveTrack()
	case "S":
		return m.switchToProvider("spotify")

	case "m":
		m.player.ToggleMono()

	case "/":
		m.search.active = true
		m.search.query = ""
		m.search.results = nil
		m.search.cursor = 0
		m.prevFocus = m.focus
		m.focus = focusSearch

	case "ctrl+f":
		m.openProviderSearch()

	case "J":
		return m.switchToProvider("jellyfin")
	case "E":
		return m.switchToProvider("emby")

	case "t":
		m.openThemePicker()

	case "i":
		m.showInfo = true

	case "u":
		m.urlInputting = true
		m.urlInput = ""

	case "L":
		return m.switchToProvider("local")
	case "R":
		return m.switchToProvider("radio")
	case "P":
		return m.switchToProvider("plex")
	case "Y":
		return m.switchToProvider("yt")
	case "C":
		return m.switchToProvider("soundcloud")

	case "v":
		m.vis.CycleMode()
		m.applyHeightMode()
		m.adjustScroll()
		if err := m.configSaver.Save("visualizer", fmt.Sprintf("%q", m.vis.ModeName())); err != nil {
			m.status.Showf(statusTTLDefault, "Config save failed: %s", err)
		}

	case "V":
		m.fullVis = !m.fullVis
		if m.fullVis {
			m.vis.Rows = max(ui.DefaultVisRows, (m.height-10)*4/5)
			ui.PanelWidth = max(0, m.width-2*ui.PaddingH)
		} else {
			m.vis.Rows = ui.DefaultVisRows
			m.restorePanelWidth()
		}

	case "ctrl+x":
		if m.focus == focusPlaylist {
			m.toggleExpandedView()
		}

	case "x":
		if m.focus == focusPlaylist {
			m.removeSelectedFromPlaylist()
		}

	case "d":
		m.devicePicker.visible = true
		m.devicePicker.cursor = 0
		if len(m.devicePicker.devices) == 0 {
			m.devicePicker.loading = true
			return listDevicesCmd()
		}

	case "]":
		m.changeSpeed(0.25)

	case "[":
		m.changeSpeed(-0.25)

	case "ctrl+k", "?":
		m.openKeymap()

	default:
		if m.luaMgr != nil {
			m.luaMgr.EmitKey(msg.String())
		}
	}

	return nil
}

// saveTrack copies the current track to ~/Music/cliamp/ with a clean filename.
// For yt-dlp tracks (piped streams), triggers an async download via yt-dlp.
// For local temp files, copies synchronously.
func (m *Model) saveTrack() tea.Cmd {
	track, idx := m.playlist.Current()
	if idx < 0 {
		m.status.Show("Nothing to save", statusTTLShort)
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		m.status.Showf(statusTTLShort, "Save failed: %s", err)
		return nil
	}

	saveDir := filepath.Join(home, "Music", "cliamp")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		m.status.Showf(statusTTLShort, "Save failed: %s", err)
		return nil
	}

	// Radio streams cannot be saved.
	if track.Stream {
		m.status.Show("Radio streams cannot be saved", statusTTLShort)
		return nil
	}

	ext := filepath.Ext(track.Path)
	name := track.Title
	if track.Artist != "" {
		name = track.Artist + " - " + name
	}
	// Sanitize filename: remove path separators and other problematic chars.
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)

	dest := filepath.Join(saveDir, name+ext)

	if err := fileutil.CopyFile(track.Path, dest); err != nil {
		m.status.Showf(statusTTLShort, "Save failed: %s", err)
		return nil
	}

	m.status.Showf(statusTTLDefault, "Saved to ~/Music/cliamp/%s", name+ext)
	return nil
}

// openProviderSearch opens the active provider's catalog search overlay if supported.
func (m *Model) openProviderSearch() {
	m.openProviderSearchWith(m.provider)
}

// openProviderSearchWith opens a search overlay against the given provider.
func (m *Model) openProviderSearchWith(_ playlist.Provider) {
	// Native provider search is handled via the '/' key in focusProvider mode.
}

// handleProvSearchKey processes key presses while filtering the provider playlist list.
// For the radio provider, Enter fires an API search; for others, Enter loads the
// selected result. Esc cancels and restores the normal catalog view.
func (m *Model) handleProvSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	// Catalog search: API-based search (no live client-side filtering).
	if cs, ok := m.provider.(provider.CatalogSearcher); ok {
		return m.handleCatalogSearchKey(msg, cs)
	}
	switch msg.Code {
	case tea.KeyEscape:
		m.provSearch.active = false
	case tea.KeyEnter:
		if len(m.provSearch.results) > 0 && !m.provLoading {
			idx := m.provSearch.results[m.provSearch.cursor]
			m.provCursor = idx
			m.providerMaybeAdjustScroll()
			m.provLoading = true
			m.provSearch.active = false
			m.activeProviderPlaylistID = m.providerLists[idx].ID
			return fetchTracksCmd(m.provider, m.providerLists[idx].ID)
		}
	case tea.KeyUp:
		if m.provSearch.cursor > 0 {
			m.provSearch.cursor--
		}
	case tea.KeyDown:
		if m.provSearch.cursor < len(m.provSearch.results)-1 {
			m.provSearch.cursor++
		}
	case tea.KeyBackspace:
		if m.provSearch.query != "" {
			m.provSearch.query = removeLastRune(m.provSearch.query)
			m.updateProvSearch()
		}
	case tea.KeySpace:
		m.provSearch.query += " "
		m.updateProvSearch()
	default:
		if len(msg.Text) > 0 {
			m.provSearch.query += msg.Text
			m.updateProvSearch()
		}
	}
	return nil
}

// handleCatalogSearchKey handles search input for providers with catalog search.
// Types a query, Enter fires API search, Esc cancels/clears.
func (m *Model) handleCatalogSearchKey(msg tea.KeyPressMsg, cs provider.CatalogSearcher) tea.Cmd {
	switch msg.Code {
	case tea.KeyEscape:
		m.provSearch.active = false
		m.restoreCatalog(cs)
	case tea.KeyEnter:
		m.provSearch.active = false
		if m.provSearch.query == "" {
			m.restoreCatalog(cs)
			return nil
		}
		m.provLoading = true
		return fetchCatalogSearchCmd(cs, m.provSearch.query)
	case tea.KeyBackspace, tea.KeyDelete:
		if m.provSearch.query != "" {
			m.provSearch.query = removeLastRune(m.provSearch.query)
		}
	case tea.KeySpace:
		m.provSearch.query += " "
	default:
		if len(msg.Text) > 0 {
			m.provSearch.query += msg.Text
		}
	}
	return nil
}

// restoreCatalog clears search results and restores the normal catalog view.
func (m *Model) restoreCatalog(cs provider.CatalogSearcher) {
	if !cs.IsSearching() {
		return
	}
	cs.ClearSearch()
	if lists, err := m.provider.Playlists(); err == nil {
		m.providerLists = lists
	}
	m.provCursor = 0
	m.provScroll = 0
}

func (m *Model) updateProvSearch() {
	m.provSearch.results = nil
	m.provSearch.cursor = 0
	if m.provSearch.query == "" {
		return
	}
	q := strings.ToLower(m.provSearch.query)
	for i, pl := range m.providerLists {
		if strings.Contains(strings.ToLower(pl.Name), q) {
			m.provSearch.results = append(m.provSearch.results, i)
		}
	}
}

// toggleExpandedView toggles the UI between default and expanded height.
func (m *Model) toggleExpandedView() {
	m.heightExpanded = !m.heightExpanded
	m.applyHeightMode()
	m.adjustScroll()
}

// handlePaste routes pasted text to the active text input field.
// The priority order mirrors handleKey so the correct input receives the content.
func (m *Model) handlePaste(content string) tea.Cmd {
	if content == "" {
		return nil
	}

	// Keymap overlay search
	if m.keymap.visible {
		m.keymap.search += content
		m.updateKeymapFilter()
		return nil
	}

	if m.urlInputting {
		m.urlInput += content
		return nil
	}

	if m.search.active {
		m.search.query += content
		m.updateSearch()
		return nil
	}

	if m.provSearch.active {
		m.provSearch.query += content
		if _, ok := m.provider.(provider.CatalogSearcher); !ok {
			m.updateProvSearch()
		}
		return nil
	}

	return nil
}

func (m *Model) handleSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	// Allow opening overlays during search (ctrl combos don't conflict with text input).
	switch msg.String() {
	case "ctrl+k":
		m.openKeymap()
		return nil
	}

	switch msg.Code {
	case tea.KeyEscape:
		m.search.active = false
		m.focus = m.prevFocus

	case tea.KeyEnter:
		var cmd tea.Cmd
		if len(m.search.results) > 0 {
			idx := m.search.results[m.search.cursor]
			m.playlist.SetIndex(idx)
			m.plCursor = idx
			m.adjustScroll()
			cmd = m.playCurrentTrack()
			m.notifyPlayback()
		}
		m.search.active = false
		m.focus = focusPlaylist
		return cmd

	case tea.KeyTab:

	case tea.KeyUp:
		if m.search.cursor > 0 {
			m.search.cursor--
		}

	case tea.KeyDown:
		if m.search.cursor < len(m.search.results)-1 {
			m.search.cursor++
		}

	case tea.KeyBackspace:
		if m.search.query != "" {
			m.search.query = removeLastRune(m.search.query)
			m.updateSearch()
		}

	case tea.KeySpace:
		m.search.query += " "
		m.updateSearch()

	default:
		if len(msg.Text) > 0 {
			m.search.query += msg.Text
			m.updateSearch()
		}
	}

	return nil
}

// handleURLInputKey processes key presses while in URL input mode.
func (m *Model) handleURLInputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEscape:
		m.urlInputting = false
	case tea.KeyEnter:
		m.urlInputting = false
		input := strings.TrimSpace(m.urlInput)
		if input != "" {
			m.feedLoading = true
			m.status.Show("Loading URL...", statusTTLLong)
			return resolveRemoteCmd([]string{input}, true)
		}
	case tea.KeyBackspace:
		m.urlInput = removeLastRune(m.urlInput)
	default:
		if len(msg.Text) > 0 {
			m.urlInput += msg.Text
		}
	}
	return nil
}

// handleThemeKey processes key presses while the theme picker is open.
func (m *Model) handleThemeKey(msg tea.KeyPressMsg) tea.Cmd {
	count := len(m.themes) + 1 // +1 for Default
	switch msg.String() {
	case "ctrl+c":
		m.themePickerCancel()
		return m.quit()

	case "up", "k":
		if m.themePicker.cursor > 0 {
			m.themePicker.cursor--
		} else if count > 0 {
			m.themePicker.cursor = count - 1
		}
		m.themePickerApply()
		m.themePickerMaybeAdjustScroll(m.themePickerVisible())

	case "down", "j":
		if m.themePicker.cursor < count-1 {
			m.themePicker.cursor++
		} else if count > 0 {
			m.themePicker.cursor = 0
		}
		m.themePickerApply()
		m.themePickerMaybeAdjustScroll(m.themePickerVisible())

	case "ctrl+x":
		m.toggleExpandedView()
		m.themePickerMaybeAdjustScroll(m.themePickerVisible())

	case "pgup", "ctrl+u":
		if m.themePicker.cursor > 0 {
			visible := m.themePickerVisible()
			m.themePicker.cursor -= min(m.themePicker.cursor, visible)
			m.themePickerApply()
			m.themePickerMaybeAdjustScroll(visible)
		}

	case "pgdown", "ctrl+d":
		if m.themePicker.cursor < count-1 {
			visible := m.themePickerVisible()
			m.themePicker.cursor = min(count-1, m.themePicker.cursor+visible)
			m.themePickerApply()
			m.themePickerMaybeAdjustScroll(visible)
		}

	case "home", "g":
		m.themePicker.cursor = 0
		m.themePickerApply()
		m.themePickerMaybeAdjustScroll(m.themePickerVisible())

	case "end", "G":
		if count > 0 {
			m.themePicker.cursor = count - 1
		}
		m.themePickerApply()
		m.themePickerMaybeAdjustScroll(m.themePickerVisible())

	case "enter":
		m.themePickerSelect()

	case "esc", "q", "t":
		m.themePickerCancel()
	}
	return nil
}

// handleDeviceKey processes key presses while the audio device picker is open.
func (m *Model) handleDeviceKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		m.devicePicker.visible = false
		return m.quit()
	case "up", "k":
		if m.devicePicker.cursor > 0 {
			m.devicePicker.cursor--
		} else if len(m.devicePicker.devices) > 0 {
			m.devicePicker.cursor = len(m.devicePicker.devices) - 1
		}
	case "down", "j":
		if m.devicePicker.cursor < len(m.devicePicker.devices)-1 {
			m.devicePicker.cursor++
		} else {
			m.devicePicker.cursor = 0
		}
	case "enter":
		if len(m.devicePicker.devices) > 0 && m.devicePicker.cursor < len(m.devicePicker.devices) {
			dev := m.devicePicker.devices[m.devicePicker.cursor]
			m.devicePicker.visible = false
			return switchDeviceCmd(dev.Name)
		}
	case "esc", "d":
		m.devicePicker.visible = false
	}
	return nil
}
