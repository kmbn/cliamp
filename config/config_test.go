package config

import (
	"testing"
)

func TestClampVolume(t *testing.T) {
	tests := []struct {
		name string
		vol  float64
		want float64
	}{
		{"within range", -10, -10},
		{"too low", -50, -30},
		{"too high", 20, 6},
		{"min boundary", -30, -30},
		{"max boundary", 6, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Volume = tt.vol
			cfg.clamp()
			if cfg.Volume != tt.want {
				t.Errorf("Volume = %f, want %f", cfg.Volume, tt.want)
			}
		})
	}
}

func TestClampSampleRate(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},         // auto-detect preserved
		{44100, 44100}, // exact match
		{48000, 48000},
		{96000, 96000},
		{30000, 22050}, // rounds to nearest
		{45000, 44100},
		{50000, 48000},
		{100000, 96000},
		{200000, 192000},
	}
	for _, tt := range tests {
		got := clampSampleRate(tt.input)
		if got != tt.want {
			t.Errorf("clampSampleRate(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestClampBitDepth(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{8, 16},
		{16, 16},
		{24, 32},
		{32, 32},
	}
	for _, tt := range tests {
		got := clampBitDepth(tt.input)
		if got != tt.want {
			t.Errorf("clampBitDepth(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestClampBufferMs(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{100, 100},
		{10, 50},
		{600, 500},
		{50, 50},
		{500, 500},
	}
	for _, tt := range tests {
		cfg := defaultConfig()
		cfg.BufferMs = tt.input
		cfg.clamp()
		if cfg.BufferMs != tt.want {
			t.Errorf("BufferMs(%d) = %d, want %d", tt.input, cfg.BufferMs, tt.want)
		}
	}
}

func TestClampResampleQuality(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{1, 1},
		{4, 4},
		{5, 4},
	}
	for _, tt := range tests {
		cfg := defaultConfig()
		cfg.ResampleQuality = tt.input
		cfg.clamp()
		if cfg.ResampleQuality != tt.want {
			t.Errorf("ResampleQuality(%d) = %d, want %d", tt.input, cfg.ResampleQuality, tt.want)
		}
	}
}

func TestClampPadding(t *testing.T) {
	tests := []struct {
		name         string
		inH, inV     int
		wantH, wantV int
	}{
		{"negative clamped to 0", -1, -1, 0, 0},
		{"over max clamped", 20, 10, 10, 5},
		{"within range", 3, 1, 3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.PaddingH = tt.inH
			cfg.PaddingV = tt.inV
			cfg.clamp()
			if cfg.PaddingH != tt.wantH {
				t.Errorf("PaddingH = %d, want %d", cfg.PaddingH, tt.wantH)
			}
			if cfg.PaddingV != tt.wantV {
				t.Errorf("PaddingV = %d, want %d", cfg.PaddingV, tt.wantV)
			}
		})
	}
}

func TestParseEQ(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want [10]float64
	}{
		{
			name: "all zeros",
			val:  "[0, 0, 0, 0, 0, 0, 0, 0, 0, 0]",
			want: [10]float64{},
		},
		{
			name: "mixed values",
			val:  "[3, -2, 0, 1.5, -5, 0, 0, 0, 0, 0]",
			want: [10]float64{3, -2, 0, 1.5, -5, 0, 0, 0, 0, 0},
		},
		{
			name: "clamped to range",
			val:  "[15, -20, 0, 0, 0, 0, 0, 0, 0, 0]",
			want: [10]float64{12, -12, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "fewer than 10",
			val:  "[1, 2, 3]",
			want: [10]float64{1, 2, 3, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "empty",
			val:  "[]",
			want: [10]float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEQ(tt.val)
			if got != tt.want {
				t.Errorf("parseEQ(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestOverridesApply(t *testing.T) {
	cfg := defaultConfig()

	vol := -15.0
	mono := true
	theme := "dark"
	compact := true
	sr := 48000
	play := true

	overrides := Overrides{
		Volume:     &vol,
		Mono:       &mono,
		Theme:      &theme,
		Compact:    &compact,
		SampleRate: &sr,
		Play:       &play,
	}

	overrides.Apply(&cfg)

	if cfg.Volume != -15 {
		t.Errorf("Volume = %f, want -15", cfg.Volume)
	}
	if !cfg.Mono {
		t.Error("Mono should be true")
	}
	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want dark", cfg.Theme)
	}
	if !cfg.Compact {
		t.Error("Compact should be true")
	}
	if cfg.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cfg.SampleRate)
	}
	if !cfg.AutoPlay {
		t.Error("AutoPlay should be true")
	}
}

func TestOverridesApplyNil(t *testing.T) {
	cfg := defaultConfig()
	original := cfg

	overrides := Overrides{} // all nil
	overrides.Apply(&cfg)

	// Nothing should change except clamp effects
	if cfg.Volume != original.Volume {
		t.Error("nil overrides changed Volume")
	}
}

func TestOverridesApplyClamps(t *testing.T) {
	cfg := defaultConfig()

	vol := 100.0 // out of range
	overrides := Overrides{Volume: &vol}
	overrides.Apply(&cfg)

	if cfg.Volume != 6 {
		t.Errorf("Volume should be clamped to 6, got %f", cfg.Volume)
	}
}

// Mock player for ApplyPlayer tests
type mockPlayer struct {
	volume float64
	eq     [10]float64
	mono   bool
}

func (m *mockPlayer) SetVolume(db float64)           { m.volume = db }
func (m *mockPlayer) SetEQBand(band int, dB float64) { m.eq[band] = dB }
func (m *mockPlayer) ToggleMono()                    { m.mono = !m.mono }

func TestApplyPlayer(t *testing.T) {
	cfg := defaultConfig()
	cfg.Volume = -10
	cfg.EQ = [10]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cfg.EQPreset = "" // Custom
	cfg.Mono = true

	p := &mockPlayer{}
	cfg.ApplyPlayer(p)

	if p.volume != -10 {
		t.Errorf("volume = %f, want -10", p.volume)
	}
	for i, want := range cfg.EQ {
		if p.eq[i] != want {
			t.Errorf("eq[%d] = %f, want %f", i, p.eq[i], want)
		}
	}
	if !p.mono {
		t.Error("mono should be true")
	}
}

func TestApplyPlayerWithPreset(t *testing.T) {
	cfg := defaultConfig()
	cfg.EQPreset = "Rock"
	cfg.EQ = [10]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	p := &mockPlayer{}
	cfg.ApplyPlayer(p)

	// With a preset set (not Custom/""), individual EQ bands should NOT be applied
	for i, v := range p.eq {
		if v != 0 {
			t.Errorf("eq[%d] = %f, want 0 (preset should skip band apply)", i, v)
		}
	}
}
