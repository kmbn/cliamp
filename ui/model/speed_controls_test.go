package model

import (
	"cliamp/config"
	"cliamp/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTickPendingSpeedSaveUsesElapsedTime(t *testing.T) {
	if sharedPlayer == nil {
		t.Skip("audio hardware unavailable")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	sharedPlayer.Stop()
	origSpeed := sharedPlayer.Speed()
	sharedPlayer.SetSpeed(1.0)
	t.Cleanup(func() {
		sharedPlayer.SetSpeed(origSpeed)
	})

	m := Model{player: sharedPlayer, configSaver: config.SaveFunc{}}
	m.changeSpeed(0.5)

	configPath := filepath.Join(home, ".config", "cliamp", "config.toml")
	for i := range 4 {
		m.tickPendingSpeedSave(ui.TickSlow)
		if _, err := os.Stat(configPath); !os.IsNotExist(err) {
			t.Fatalf("config created after %d slow ticks, want no save before %v", i+1, speedSaveDebounce)
		}
	}

	m.tickPendingSpeedSave(ui.TickSlow)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", configPath, err)
	}
	if got := string(data); !strings.Contains(got, "speed = 1.50") {
		t.Fatalf("config contents = %q, want speed = 1.50", got)
	}
	if got := m.speedSaveAfter; got != 0 {
		t.Fatalf("speedSaveAfter after save = %v, want 0", got)
	}
}

func TestFlushPendingSpeedSavePersistsImmediately(t *testing.T) {
	if sharedPlayer == nil {
		t.Skip("audio hardware unavailable")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	sharedPlayer.Stop()
	origSpeed := sharedPlayer.Speed()
	sharedPlayer.SetSpeed(1.0)
	t.Cleanup(func() {
		sharedPlayer.SetSpeed(origSpeed)
	})

	m := Model{player: sharedPlayer, configSaver: config.SaveFunc{}}
	m.changeSpeed(0.25)
	m.flushPendingSpeedSave()

	configPath := filepath.Join(home, ".config", "cliamp", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", configPath, err)
	}
	if got := string(data); !strings.Contains(got, "speed = 1.25") {
		t.Fatalf("config contents = %q, want speed = 1.25", got)
	}
	if got := m.speedSaveAfter; got != 0 {
		t.Fatalf("speedSaveAfter after flush = %v, want 0", got)
	}
}
