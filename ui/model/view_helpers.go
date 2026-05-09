package model

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// formatTrackTime formats a duration in seconds as M:SS or H:MM:SS for tracks.
// Returns "" when secs is non-positive so callers can skip rendering entirely.
func formatTrackTime(secs int) string {
	if secs <= 0 {
		return ""
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// formatPlaylistDuration formats a total runtime for a playlist as "1h 23m"
// or "12m" or "45s". Returns "" when secs is non-positive.
func formatPlaylistDuration(secs int) string {
	if secs <= 0 {
		return ""
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	if h > 0 {
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", secs)
}

// case where the string is already short enough.
func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:maxW-1]) + "…"
}

// cursorLine renders a list item with "> " prefix when active, "  " otherwise.
func cursorLine(label string, active bool) string {
	if active {
		return playlistSelectedStyle.Render("> " + label)
	}
	return dimStyle.Render("  " + label)
}

// scrollStart returns the scroll offset so that cursor remains visible
// within a window of maxVisible items.
func scrollStart(cursor, maxVisible int) int {
	if cursor >= maxVisible {
		return cursor - maxVisible + 1
	}
	return 0
}

// spinnerFrames is the braille-dot animation used to indicate loading. The
// view re-renders on the model tick so the spinner advances on its own.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerFrame returns the current animation frame, time-driven so the caller
// doesn't need to track an animation index.
func spinnerFrame() string {
	idx := (time.Now().UnixMilli() / 100) % int64(len(spinnerFrames))
	return spinnerFrames[idx]
}

// loadingLine renders a single styled "<spinner> <label>" line for use as a
// loading indicator inside a list pane.
func loadingLine(label string) string {
	return activeToggle.Render("  "+spinnerFrame()) + dimStyle.Render(" "+label)
}

// padLines appends empty strings so that rendered items fill maxVisible rows.
func padLines(lines []string, maxVisible, rendered int) []string {
	for range maxVisible - rendered {
		lines = append(lines, "")
	}
	return lines
}

// fitLines truncates lines to budget then pads with empty strings to exactly budget rows.
func fitLines(lines []string, budget int) []string {
	if len(lines) > budget {
		lines = lines[:budget]
	}
	return padLines(lines, budget, len(lines))
}

// helpKey renders a key as a pill (background-highlighted) followed by a dim label.
func helpKey(key, label string) string {
	return helpKeyStyle.Render(" "+key+" ") + helpStyle.Render(" "+label)
}

