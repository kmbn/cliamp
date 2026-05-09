package model

import (
	tea "charm.land/bubbletea/v2"
)

// resetProviderNav resets provider navigation and search state to the top.
func (m *Model) resetProviderNav() {
	m.provCursor = 0
	m.provScroll = 0
	m.provLoading = true
	m.provSearch.active = false
	m.provSearch.query = ""
	m.provSearch.results = nil
	m.provSearch.cursor = 0
}

// StartInProvider configures the model to begin in the provider browse view.
// Call this from main when no CLI tracks or pending URLs were given.
func (m *Model) StartInProvider() {
	if m.provider != nil {
		m.focus = focusProvider
		m.stationsVisible = true
		m.resetProviderNav()
	}
}

// switchProvider sets the active provider by pill index and fetches its playlists.
func (m *Model) switchProvider(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.providers) {
		return nil
	}
	m.provPillIdx = idx
	m.provider = m.providers[idx].Provider
	m.providerLists = nil
	m.provSignIn = false
	m.catalogBatch = catalogBatchState{}
	m.activeProviderPlaylistID = ""
	m.resetProviderNav()
	m.focus = focusProvider
	m.stationsVisible = true
	return fetchPlaylistsCmd(m.provider)
}

// quickSwitchProvider closes any browser overlays and jumps to the provider
// matched by key. Use the same Shift+letter shortcuts that switch providers
// from the main pane (S, N, P, J, Y, R, L). Returns nil when the key doesn't
// match a known provider.
func (m *Model) quickSwitchProvider(key string) tea.Cmd {
	provKey := providerKeyForShortcut(key)
	if provKey == "" {
		return nil
	}
	return m.switchToProvider(provKey)
}

// providerKeyForShortcut maps the Shift+letter provider shortcuts to the
// config key used by switchToProvider, or "" when the key is unrelated.
func providerKeyForShortcut(key string) string {
	switch key {
	case "S":
		return "spotify"
	case "N":
		return "navidrome"
	case "P":
		return "plex"
	case "J":
		return "jellyfin"
	case "E":
		return "emby"
	case "Y":
		return "yt"
	case "L":
		return "local"
	case "R":
		return "radio"
	}
	return ""
}

// switchToProvider finds a provider by config key and switches to it.
// Returns nil if the provider is not configured.
func (m *Model) switchToProvider(key string) tea.Cmd {
	for i, pe := range m.providers {
		if pe.Key == key {
			return m.switchProvider(i)
		}
	}
	return nil
}

// SetPendingURLs stores remote URLs (feeds, M3U) for async resolution after Init.
func (m *Model) SetPendingURLs(urls []string) {
	m.pendingURLs = urls
	m.feedLoading = len(urls) > 0
}

// openStations shows the station browser overlay. If the provider list has not
// been loaded yet and a fetch is not already in flight, it triggers one.
func (m *Model) openStations() tea.Cmd {
	m.stationsVisible = true
	m.focus = focusProvider
	if m.provider != nil && !m.provLoading && len(m.providerLists) == 0 {
		m.provLoading = true
		return fetchPlaylistsCmd(m.provider)
	}
	return nil
}

// closeStations hides the station browser overlay and returns focus to the
// playlist area, clearing any active provider search.
func (m *Model) closeStations() {
	m.stationsVisible = false
	m.focus = focusPlaylist
	m.provSearch.active = false
	m.provSearch.query = ""
}
