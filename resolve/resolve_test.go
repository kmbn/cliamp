package resolve

import (
	"testing"
)

func TestParseItunesDuration(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"3600", 3600},
		{"90", 90},
		{"0", 0},
		{"3661.5", 3661},
		{"90.9", 90},
		{"1:30", 90},
		{"87:05", 5225},
		{"1:27:05", 5225},
		{"0:01:30", 90},
		{" 3600 ", 3600},
		{"", 0},
		{"abc", 0},
		{"12:xx", 0},
		{"1:2:xx", 0},
		{"-1", 0},
		{"0:-10", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseItunesDuration(tt.input)
			if got != tt.want {
				t.Errorf("parseItunesDuration(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
