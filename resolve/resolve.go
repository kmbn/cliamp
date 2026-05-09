// Package resolve converts CLI arguments (stream URLs, M3U playlists, PLS
// playlists, and RSS feeds) into a flat list of playlist tracks.
package resolve

import (
	"encoding/xml"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cliamp/player"
	"cliamp/playlist"
)

// httpClient is used for feed and M3U resolution. It has a generous but
// finite timeout to prevent hanging on unresponsive servers.
var httpClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &uaTransport{rt: http.DefaultTransport},
}

// uaTransport injects the cliamp User-Agent header into every request.
type uaTransport struct{ rt http.RoundTripper }

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", "cliamp/1.0 (https://github.com/bjarneo/cliamp)")
	return t.rt.RoundTrip(req)
}

// Result holds the output of Args: instantly-resolved tracks and
// remote URLs (feeds, M3U, PLS) that need async HTTP fetching.
type Result struct {
	Tracks  []playlist.Track // plain stream URLs
	Pending []string         // feed/M3U/PLS URLs to resolve asynchronously
}

// Args separates CLI arguments into immediately-resolved stream tracks
// and pending remote URLs (feeds, M3U, PLS) that require HTTP fetching.
// Non-URL arguments are ignored in radio-only mode.
func Args(args []string) (Result, error) {
	var r Result
	for _, arg := range args {
		if !playlist.IsURL(arg) {
			continue
		}
		if playlist.IsFeed(arg) || playlist.IsM3U(arg) || playlist.IsPLS(arg) || sniffFeedURL(arg) {
			r.Pending = append(r.Pending, arg)
		} else {
			r.Tracks = append(r.Tracks, playlist.TrackFromPath(arg))
		}
	}
	return r, nil
}

// Remote fetches feed and M3U/PLS URLs and returns the resolved tracks.
func Remote(urls []string) ([]playlist.Track, error) {
	var tracks []playlist.Track
	for _, u := range urls {
		switch {
		case playlist.IsFeed(u):
			t, err := resolveFeed(u)
			if err != nil {
				return nil, fmt.Errorf("resolving feed %s: %w", u, err)
			}
			tracks = append(tracks, t...)
		case playlist.IsM3U(u):
			t, err := resolveM3U(u)
			if err != nil {
				return nil, fmt.Errorf("resolving m3u %s: %w", u, err)
			}
			tracks = append(tracks, t...)
		case playlist.IsPLS(u):
			t, err := resolvePLS(u)
			if err != nil {
				return nil, fmt.Errorf("resolving pls %s: %w", u, err)
			}
			tracks = append(tracks, t...)
		default:
			t, err := resolveFeed(u)
			if err != nil {
				return nil, fmt.Errorf("resolving feed %s: %w", u, err)
			}
			tracks = append(tracks, t...)
		}
	}
	return tracks, nil
}

// sniffFeedURL does a HEAD request and returns true if the Content-Type
// indicates an RSS/Atom feed.
func sniffFeedURL(rawURL string) bool {
	if u, err := url.Parse(rawURL); err == nil {
		if player.SupportedExts[strings.ToLower(filepath.Ext(u.Path))] {
			return false
		}
	}
	resp, err := httpClient.Head(rawURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)
	switch mediaType {
	case "application/rss+xml", "application/atom+xml",
		"application/xml", "text/xml":
		return true
	}
	return false
}

// resolveFeed fetches a podcast RSS feed and returns tracks with metadata.
func resolveFeed(feedURL string) ([]playlist.Track, error) {
	resp, err := httpClient.Get(feedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.Status)
	}

	var rss struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title     string `xml:"title"`
				Duration  string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
				Enclosure struct {
					URL  string `xml:"url,attr"`
					Type string `xml:"type,attr"`
				} `xml:"enclosure"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
		return nil, fmt.Errorf("parsing feed: %w", err)
	}

	var tracks []playlist.Track
	for _, item := range rss.Channel.Items {
		if item.Enclosure.URL == "" {
			continue
		}
		tracks = append(tracks, playlist.Track{
			Path:         item.Enclosure.URL,
			Title:        item.Title,
			Artist:       rss.Channel.Title,
			Stream:       true,
			DurationSecs: parseItunesDuration(item.Duration),
		})
	}
	return tracks, nil
}

// resolveM3U fetches an M3U playlist URL and returns tracks with EXTINF metadata.
func resolveM3U(m3uURL string) ([]playlist.Track, error) {
	resp, err := httpClient.Get(m3uURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.Status)
	}

	entries, err := parseM3U(resp.Body, "")
	if err != nil {
		return nil, err
	}
	return entriesToTracks(entries), nil
}

// resolvePLS fetches a PLS playlist URL and returns tracks.
func resolvePLS(plsURL string) ([]playlist.Track, error) {
	resp, err := httpClient.Get(plsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.Status)
	}

	entries, err := parsePLS(resp.Body)
	if err != nil {
		return nil, err
	}
	return plsEntriesToTracks(entries), nil
}

// parseItunesDuration parses an <itunes:duration> value into seconds.
func parseItunesDuration(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parseSec := func(s string) (int, error) {
		f, err := strconv.ParseFloat(s, 64)
		return int(f), err
	}
	parts := strings.Split(s, ":")
	var result int
	switch len(parts) {
	case 1:
		n, err := parseSec(parts[0])
		if err != nil {
			return 0
		}
		result = n
	case 2:
		m, err1 := strconv.Atoi(parts[0])
		sec, err2 := parseSec(parts[1])
		if err1 != nil || err2 != nil {
			return 0
		}
		result = m*60 + sec
	case 3:
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		sec, err3 := parseSec(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0
		}
		result = h*3600 + m*60 + sec
	default:
		return 0
	}
	if result < 0 {
		return 0
	}
	return result
}

// humanizeBasename converts a URL basename like "clr-podcast-467" into "clr podcast 467".
func humanizeBasename(s string) string {
	return strings.ReplaceAll(s, "-", " ")
}
