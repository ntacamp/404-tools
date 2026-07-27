package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

const validYAML = `
schedule_url: "https://example.com/schedule.csv"
channel_id: "123456789012345678"
reminder_offset_minutes: 10
poll_interval: 30s
timezone: "Europe/Vilnius"
`

func TestLoad_Valid(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	path := writeTempConfig(t, validYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PollInterval.Duration != 30*time.Second {
		t.Errorf("poll interval = %v, want 30s", cfg.PollInterval.Duration)
	}
	if cfg.BotToken != "test-token" {
		t.Errorf("bot token = %q, want %q", cfg.BotToken, "test-token")
	}
	if cfg.Location == nil || cfg.Location.String() != "Europe/Vilnius" {
		t.Errorf("location = %v, want Europe/Vilnius", cfg.Location)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "")
	path := writeTempConfig(t, validYAML)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing DISCORD_BOT_TOKEN, got nil")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	path := writeTempConfig(t, `
schedule_url: "https://example.com/schedule.csv"
channel_id: "123456789012345678"
reminder_offset_minutes: 10
poll_interval: "not-a-duration"
timezone: "Europe/Vilnius"
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid poll_interval, got nil")
	}
}
