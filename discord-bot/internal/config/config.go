// Package config loads the bot's YAML configuration and the Discord bot
// token from the environment.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ntacamp/404-tools/discord-bot/internal/schedule"
)

// Duration wraps time.Duration so it can be unmarshaled from a plain YAML
// string such as "30s" or "1m" via time.ParseDuration.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

// Config holds all settings needed to run the bot.
type Config struct {
	// ScheduleURL is either an http(s) URL or a local file path; see
	// schedule.IsURL for how that's decided.
	ScheduleURL           string   `yaml:"schedule_url"`
	ChannelID             string   `yaml:"channel_id"`
	ReminderOffsetMinutes int      `yaml:"reminder_offset_minutes"`
	PollInterval          Duration `yaml:"poll_interval"`
	Timezone              string   `yaml:"timezone"`

	// BotToken and Location are not read from YAML: the token comes from an
	// environment variable (so it never ends up committed alongside a public
	// config file), and Location is resolved from Timezone at load time.
	BotToken string         `yaml:"-"`
	Location *time.Location `yaml:"-"`
}

// Load reads and validates the config file at path, and fills in the bot
// token from the DISCORD_BOT_TOKEN environment variable.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.BotToken = os.Getenv("DISCORD_BOT_TOKEN")

	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("loading timezone %q: %w", cfg.Timezone, err)
		}
		cfg.Location = loc
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.ScheduleURL == "" {
		return fmt.Errorf("schedule_url is required")
	}
	if !schedule.IsURL(c.ScheduleURL) {
		if _, err := os.Stat(c.ScheduleURL); err != nil {
			return fmt.Errorf("schedule_url is not a valid URL and not a readable local file: %w", err)
		}
	}
	if c.ChannelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if c.ReminderOffsetMinutes <= 0 {
		return fmt.Errorf("reminder_offset_minutes must be greater than 0")
	}
	if c.PollInterval.Duration <= 0 {
		return fmt.Errorf("poll_interval must be greater than 0")
	}
	if c.Timezone == "" {
		return fmt.Errorf("timezone is required")
	}
	if c.BotToken == "" {
		return fmt.Errorf("DISCORD_BOT_TOKEN environment variable is required")
	}
	return nil
}
