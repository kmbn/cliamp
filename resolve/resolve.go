// Package resolve converts CLI arguments (stream URLs, M3U playlists, PLS
// playlists, and RSS feeds) into a flat list of playlist tracks.
package resolve

import (
	"fmt"
	"net/http"
	"time"

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
		if playlist.IsM3U(arg) || playlist.IsPLS(arg) {
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
		}
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

	entries, err := parseM3U(resp.Body)
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
