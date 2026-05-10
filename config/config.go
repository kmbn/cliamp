// Package config handles loading user configuration from ~/.config/cliamp/config.toml.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cliamp/internal/appdir"
)

// configPath returns the path to the config file.
func configPath() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// parseString trims surrounding quotes from a TOML string value and, if the
// result is exactly $NAME or ${NAME}, replaces it with the value of that
// environment variable (or "" when unset). Mixed values containing other
// characters are left untouched, so literal '$' in passwords is preserved.
func parseString(s string) string {
	s = strings.Trim(s, `"'`)
	if len(s) < 2 || s[0] != '$' {
		return s
	}
	name := s[1:]
	if name[0] == '{' {
		if name[len(name)-1] != '}' {
			return s
		}
		name = name[1 : len(name)-1]
	}
	if !isEnvName(name) {
		return s
	}
	return os.Getenv(name)
}

func isEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// Config holds user preferences loaded from the config file.
type Config struct {
	Volume          float64     // dB, range [-30, +6]
	EQ              [10]float64 // per-band gain in dB, range [-12, +12]
	EQPreset        string      // preset name, or "" for custom
	Mono            bool
	AutoPlay        bool   // start playback automatically on launch (radio streams, CLI tracks)
	Theme           string // theme name, or "" for ANSI default
	Visualizer      string // visualizer mode name, or "" for default (Bars)
	SampleRate      int    // output sample rate: 22050, 44100, 48000, 96000, 192000
	BufferMs        int    // speaker buffer in milliseconds (50–500)
	ResampleQuality int    // beep resample quality factor (1–4)
	BitDepth        int    // PCM bit depth for FFmpeg output: 16 or 32
	Compact         bool   // compact mode: cap frame width at 80 columns
	PaddingH        int    // horizontal padding for the UI frame (default 3)
	PaddingV        int    // vertical padding for the UI frame (default 1)
	AudioDevice     string // preferred audio output device name (empty = system default)
	LogLevel        string // log level: debug, info, warn, error (default "info")
}

// defaultConfig returns a Config with sensible defaults.
// SampleRate defaults to 0, which means "auto-detect from the system's default
// output device" (see player.DeviceSampleRate). This ensures USB audio devices
// that require a specific rate (commonly 48 kHz) work out of the box.
func defaultConfig() Config {
	return Config{
		AutoPlay:        false,
		SampleRate:      0,
		BufferMs:        100,
		ResampleQuality: 4,
		BitDepth:        16,
		PaddingH:        3,
		PaddingV:        1,
		LogLevel:        "info",
	}
}

// Load reads the config file from ~/.config/cliamp/config.toml.
// Returns defaults if the file does not exist.
func Load() (Config, error) {
	cfg := defaultConfig()

	path, err := configPath()
	if err != nil {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	section := "" // current [section] header, empty = top-level
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line[1 : len(line)-1])
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if section != "" {
			continue // unknown section — skip
		}
		switch key {
		case "volume":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.Volume = v
			}
		case "mono":
			cfg.Mono = val == "true"
		case "auto_play":
			cfg.AutoPlay = val == "true"
		case "eq":
			cfg.EQ = parseEQ(val)
		case "eq_preset":
			cfg.EQPreset = parseString(val)
		case "theme":
			cfg.Theme = parseString(val)
		case "visualizer":
			cfg.Visualizer = parseString(val)
		case "sample_rate":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.SampleRate = v
			}
		case "buffer_ms":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.BufferMs = v
			}
		case "resample_quality":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.ResampleQuality = v
			}
		case "bit_depth":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.BitDepth = v
			}
		case "compact":
			cfg.Compact = val == "true"
		case "audio_device":
			cfg.AudioDevice = parseString(val)
		case "padding_horizontal":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.PaddingH = v
			}
		case "padding_vertical":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.PaddingV = v
			}
		case "log_level":
			lvl := strings.ToLower(parseString(val))
			switch lvl {
			case "debug", "info", "warn", "warning", "error":
				cfg.LogLevel = lvl
			}
		}
	}

	cfg.clamp()
	return cfg, scanner.Err()
}

// Save updates only the given key in the existing config file, preserving
// all other content, comments, and formatting. If the key doesn't exist,
// it is appended. If no config file exists, one is created with just that key.
func Save(key, value string) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	line := fmt.Sprintf("%s = %s", key, value)

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return os.WriteFile(path, []byte(line+"\n"), 0o644)
	}

	// Scan existing lines and replace the matching key in-place,
	// but only in the top-level scope (before any [section] header).
	lines := strings.Split(string(data), "\n")
	found := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Stop searching once we hit a section header — the key
		// belongs in the top-level scope only.
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			break
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(k) == key {
			lines[i] = line
			found = true
			break
		}
	}
	if !found {
		// Insert before the first section header to keep top-level keys together.
		inserted := false
		for i, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				lines = append(lines[:i], append([]string{line}, lines[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			lines = append(lines, line)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// PlayerConfig is the subset of player controls needed to apply config.
type PlayerConfig interface {
	SetVolume(db float64)
	SetEQBand(band int, dB float64)
	ToggleMono()
}

// ApplyPlayer applies audio-engine settings from the config.
func (c Config) ApplyPlayer(p PlayerConfig) {
	p.SetVolume(c.Volume)
	if c.EQPreset == "" || c.EQPreset == "Custom" {
		for i, gain := range c.EQ {
			p.SetEQBand(i, gain)
		}
	}
	if c.Mono {
		p.ToggleMono()
	}
}

// clamp constrains all Config fields to their valid ranges.
func (c *Config) clamp() {
	c.Volume = max(min(c.Volume, 6), -30)
	c.SampleRate = clampSampleRate(c.SampleRate)
	c.BufferMs = max(min(c.BufferMs, 500), 50)
	c.ResampleQuality = max(min(c.ResampleQuality, 4), 1)
	c.BitDepth = clampBitDepth(c.BitDepth)
	c.PaddingH = max(min(c.PaddingH, 10), 0)
	c.PaddingV = max(min(c.PaddingV, 5), 0)
}

// nearestAllowed returns the value in allowed closest to v.
// allowed must be non-empty.
func nearestAllowed(v int, allowed []int) int {
	best := allowed[0]
	bestDist := abs(v - best)
	for _, a := range allowed[1:] {
		if d := abs(v - a); d < bestDist {
			best = a
			bestDist = d
		}
	}
	return best
}

// clampSampleRate returns the nearest valid sample rate from the allowed set.
// A value of 0 is preserved as-is to signal "auto-detect" to the player.
func clampSampleRate(v int) int {
	if v == 0 {
		return 0 // auto-detect
	}
	return nearestAllowed(v, []int{22050, 44100, 48000, 96000, 192000})
}

// clampBitDepth returns the nearest valid bit depth (16 or 32).
func clampBitDepth(v int) int {
	if v >= 24 {
		return 32
	}
	return 16
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// parseEQ parses a TOML-style array like [0, 1.5, -2, ...] into 10 bands.
func parseEQ(val string) [10]float64 {
	var bands [10]float64
	val = strings.Trim(val, "[]")
	parts := strings.Split(val, ",")
	for i, p := range parts {
		if i >= 10 {
			break
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err == nil {
			bands[i] = max(min(v, 12), -12)
		}
	}
	return bands
}
