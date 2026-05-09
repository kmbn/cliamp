// Package playlist manages an ordered track list.
package playlist

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Track represents a single audio file or HTTP stream.
type Track struct {
	Path         string
	Title        string
	Artist       string
	Album        string
	Genre        string
	Year         int
	TrackNumber  int
	Stream       bool // true for HTTP/HTTPS URLs
	Realtime     bool // true for real-time/live streams (e.g. radio)
	Feed         bool // true for RSS/podcast feed URLs (resolved before playback)
	DurationSecs int  // known duration in seconds (0 = unknown)
	Bookmark     bool // user-bookmarked track

	Unplayable bool // true when the track is known not playable in the current playback context

	// ProviderMeta holds provider-specific key-value pairs.
	// Keys are namespaced by provider, e.g. "navidrome.id", "jellyfin.id".
	ProviderMeta map[string]string
}

// Meta returns the value for a provider-specific metadata key, or "" if unset.
func (t Track) Meta(key string) string {
	if t.ProviderMeta == nil {
		return ""
	}
	return t.ProviderMeta[key]
}

// TotalDurationSecs sums DurationSecs across a slice of tracks, skipping
// entries with unknown duration (zero).
func TotalDurationSecs(tracks []Track) int {
	total := 0
	for _, t := range tracks {
		if t.DurationSecs > 0 {
			total += t.DurationSecs
		}
	}
	return total
}

// IsURL reports whether path is an HTTP or HTTPS URL, or a yt-dlp search protocol string.
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") ||
		IsYTSearch(path)
}

// IsYTSearch reports whether path is a yt-dlp search expression
// (ytsearch:, ytsearchN:, scsearch:, scsearchN:).
func IsYTSearch(path string) bool {
	return matchSearchPrefix(path, "ytsearch") || matchSearchPrefix(path, "scsearch")
}

func matchSearchPrefix(path, name string) bool {
	if !strings.HasPrefix(path, name) {
		return false
	}
	rest := path[len(name):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return false
	}
	for _, c := range rest[:colon] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// IsM3U reports whether the path points to an M3U playlist file (URL or local).
func IsM3U(path string) bool {
	if IsURL(path) {
		u, err := url.Parse(path)
		if err != nil {
			return false
		}
		ext := strings.ToLower(filepath.Ext(u.Path))
		return ext == ".m3u" || ext == ".m3u8"
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".m3u" || ext == ".m3u8"
}

// IsLocalM3U reports whether the path is a local (non-URL) M3U file.
func IsLocalM3U(path string) bool {
	return !IsURL(path) && IsM3U(path)
}

// IsPLS reports whether the path points to a PLS playlist file (URL or local).
func IsPLS(path string) bool {
	if IsURL(path) {
		u, err := url.Parse(path)
		if err != nil {
			return false
		}
		return strings.ToLower(filepath.Ext(u.Path)) == ".pls"
	}
	return strings.ToLower(filepath.Ext(path)) == ".pls"
}

// IsLocalPLS reports whether the path is a local (non-URL) PLS file.
func IsLocalPLS(path string) bool {
	return !IsURL(path) && IsPLS(path)
}

// IsYouTubeURL reports whether the URL points to YouTube (youtube.com or youtu.be).
// YouTube Music (music.youtube.com) is excluded — use IsYouTubeMusicURL for that.
func IsYouTubeURL(path string) bool {
	if !IsURL(path) {
		return false
	}
	// ytsearch: protocols are handled by yt-dlp, not the native YouTube client.
	if IsYTSearch(path) {
		return false
	}
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	switch host {
	case "youtube.com", "youtu.be":
		return true
	}
	return false
}

// IsYouTubeMusicURL reports whether the URL points to YouTube Music (music.youtube.com).
// These URLs require yt-dlp rather than the native YouTube API client.
func IsYouTubeMusicURL(path string) bool {
	if !IsURL(path) {
		return false
	}
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	return host == "music.youtube.com"
}

// IsYTDL reports whether the URL points to a site supported by yt-dlp
// (YouTube, SoundCloud, Bandcamp, ytsearch: protocol, etc.).
func IsYTDL(path string) bool {
	if !IsURL(path) {
		return false
	}
	// YouTube and YouTube Music URLs are handled by yt-dlp for playback.
	if IsYouTubeURL(path) || IsYouTubeMusicURL(path) {
		return true
	}
	if IsYTSearch(path) {
		return true
	}
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	switch host {
	case "soundcloud.com",
		"bandcamp.com",
		"music.163.com",
		"bilibili.com",
		"b23.tv":
		return true
	}
	// Bilibili subdomains (e.g. space.bilibili.com)
	if strings.HasSuffix(host, ".bilibili.com") {
		return true
	}
	// Bandcamp artist subdomains (e.g. artist.bandcamp.com)
	if strings.HasSuffix(host, ".bandcamp.com") {
		return true
	}
	return false
}

// IsXiaoyuzhouEpisode reports whether the URL points to a Xiaoyuzhou episode page.
func IsXiaoyuzhouEpisode(path string) bool {
	if !IsURL(path) {
		return false
	}
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	if host != "xiaoyuzhoufm.com" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(u.Path), "/episode/")
}

// IsFeed reports whether the URL points to a podcast RSS/XML feed.
func IsFeed(path string) bool {
	if !IsURL(path) {
		return false
	}
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	return ext == ".xml" || ext == ".rss" || ext == ".atom"
}

// TrackFromPath creates a Track by parsing the filename or URL.
// For local files, embedded tags (ID3v2, Vorbis, MP4) are tried first,
// falling back to "Artist - Title" filename parsing.
func TrackFromPath(path string) Track {
	if IsURL(path) {
		return trackFromURL(path)
	}
	return readTags(path)
}

// trackFromURL creates a Track from an HTTP/HTTPS URL, extracting a clean
// display title from the URL path (ignoring query parameters).
func trackFromURL(rawURL string) Track {
	t := Track{Path: rawURL, Stream: true}

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Title = rawURL
		return t
	}

	// Extract filename from URL path
	base := filepath.Base(u.Path)
	if base != "" && base != "." && base != "/" {
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if name != "" && name != "stream" && name != "rest" {
			t.Title = name
			return t
		}
	}

	// Fallback: use hostname
	t.Title = u.Hostname()
	return t
}

// IsLive reports whether the track is a live stream (e.g. Icecast radio)
func (t Track) IsLive() bool {
	return t.Realtime
}

// DisplayName returns a formatted display string for the track.
func (t Track) DisplayName() string {
	if t.Artist != "" {
		return t.Artist + " - " + t.Title
	}
	return t.Title
}

// Playlist manages an ordered list of tracks.
type Playlist struct {
	tracks []Track
	pos    int
}

// New creates an empty Playlist.
func New() *Playlist {
	return &Playlist{}
}

// Replace clears the playlist and loads the given tracks, resetting position.
func (p *Playlist) Replace(tracks []Track) {
	p.tracks = tracks
	p.pos = 0
}

// Add appends tracks to the playlist.
func (p *Playlist) Add(tracks ...Track) {
	p.tracks = append(p.tracks, tracks...)
}

// Len returns the number of tracks.
func (p *Playlist) Len() int { return len(p.tracks) }

// Current returns the currently selected track and its index.
// Returns an empty Track and -1 if the playlist is empty.
func (p *Playlist) Current() (Track, int) {
	if len(p.tracks) == 0 {
		return Track{}, -1
	}
	return p.tracks[p.pos], p.pos
}

// Index returns the index of the current track, or -1 if the playlist is empty.
func (p *Playlist) Index() int {
	if len(p.tracks) == 0 {
		return -1
	}
	return p.pos
}

// SetIndex sets the playback position to i, clamped to valid bounds.
func (p *Playlist) SetIndex(i int) {
	if len(p.tracks) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(p.tracks) {
		i = len(p.tracks) - 1
	}
	p.pos = i
}

// Next advances to the next track. Returns false when already at the end.
func (p *Playlist) Next() (Track, bool) {
	if p.pos+1 >= len(p.tracks) {
		return Track{}, false
	}
	p.pos++
	return p.tracks[p.pos], true
}

// Prev moves to the previous track. Returns false when already at the start.
func (p *Playlist) Prev() (Track, bool) {
	if p.pos <= 0 {
		return Track{}, false
	}
	p.pos--
	return p.tracks[p.pos], true
}

// PeekNext returns the next track without advancing. Returns false when at end.
func (p *Playlist) PeekNext() (Track, bool) {
	if p.pos+1 >= len(p.tracks) {
		return Track{}, false
	}
	return p.tracks[p.pos+1], true
}

// SetTrack replaces the track at index i.
func (p *Playlist) SetTrack(i int, t Track) {
	if i >= 0 && i < len(p.tracks) {
		p.tracks[i] = t
	}
}

// Tracks returns all tracks in the playlist.
func (p *Playlist) Tracks() []Track { return p.tracks }
