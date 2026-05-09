package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"cliamp/playlist"
)

func TestHandlePasteRoutesToActiveInput(t *testing.T) {
	tests := []struct {
		name    string
		model   Model
		content string
		check   func(t *testing.T, m *Model)
	}{
		{
			name:    "keymap search",
			model:   Model{keymap: keymapOverlay{visible: true}},
			content: "ctrl",
			check: func(t *testing.T, m *Model) {
				if m.keymap.search != "ctrl" {
					t.Fatalf("keymap.search = %q, want %q", m.keymap.search, "ctrl")
				}
			},
		},
		{
			name:    "search appends and filters",
			model:   Model{search: searchState{active: true, query: "ja"}, playlist: playlist.New()},
			content: "zz",
			check: func(t *testing.T, m *Model) {
				if m.search.query != "jazz" {
					t.Fatalf("search.query = %q, want %q", m.search.query, "jazz")
				}
			},
		},
		{
			name:    "url input",
			model:   Model{urlInputting: true},
			content: "https://example.com/song.mp3",
			check: func(t *testing.T, m *Model) {
				if m.urlInput != "https://example.com/song.mp3" {
					t.Fatalf("urlInput = %q, want %q", m.urlInput, "https://example.com/song.mp3")
				}
			},
		},
		{
			name:    "provider search (non-catalog)",
			model:   Model{provSearch: provSearchState{active: true, query: "rock"}},
			content: " ballads",
			check: func(t *testing.T, m *Model) {
				if m.provSearch.query != "rock ballads" {
					t.Fatalf("provSearch.query = %q, want %q", m.provSearch.query, "rock ballads")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model
			if cmd := m.handlePaste(tt.content); cmd != nil {
				t.Fatalf("handlePaste returned non-nil cmd")
			}
			tt.check(t, &m)
		})
	}
}

func TestHandlePasteEmptyContentIsNoop(t *testing.T) {
	m := Model{search: searchState{active: true, query: "before"}, playlist: playlist.New()}

	if cmd := m.handlePaste(""); cmd != nil {
		t.Fatalf("handlePaste(\"\") returned non-nil cmd")
	}
	if m.search.query != "before" {
		t.Fatalf("query changed on empty paste: got %q", m.search.query)
	}
}

func TestHandlePasteNoInputActiveIsNoop(t *testing.T) {
	m := Model{focus: focusPlaylist}

	if cmd := m.handlePaste("ignored text"); cmd != nil {
		t.Fatalf("handlePaste returned non-nil cmd when no input active")
	}
}

func TestHandlePastePriorityOrder(t *testing.T) {
	// Keymap search has higher priority than provider search.
	m := Model{
		keymap:     keymapOverlay{visible: true},
		provSearch: provSearchState{active: true},
	}

	m.handlePaste("test")

	if m.keymap.search != "test" {
		t.Fatalf("keymap.search = %q, want %q", m.keymap.search, "test")
	}
	if m.provSearch.query != "" {
		t.Fatalf("provSearch.query = %q, want empty (lower priority)", m.provSearch.query)
	}
}

func TestUpdateRoutesPasteMsg(t *testing.T) {
	m := Model{keymap: keymapOverlay{visible: true}}

	next, cmd := m.Update(tea.PasteMsg{Content: "pasted"})
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("Update(PasteMsg) cmd = %v, want nil", cmd)
	}
	if got.keymap.search != "pasted" {
		t.Fatalf("keymap.search = %q, want %q", got.keymap.search, "pasted")
	}
}
