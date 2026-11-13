package discordbot

import (
	"testing"
	"time"
)

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{8 * time.Second, "8s"},
		{5*time.Minute + 12*time.Second, "5m 12s"},
		{time.Hour + 30*time.Second, "1h 0m"},
		{3*time.Hour + 4*time.Minute, "3h 4m"},
		{2*24*time.Hour + 4*time.Minute, "2d 0h 4m"},
		{1500 * time.Millisecond, "2s"},
	}
	for _, tt := range tests {
		if got := formatUptime(tt.in); got != tt.want {
			t.Errorf("formatUptime(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
