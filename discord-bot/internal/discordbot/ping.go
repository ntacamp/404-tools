package discordbot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/ntacamp/404-tools/discord-bot/internal/version"
)

// PingCommand implements "/ping", a simple liveness check.
type PingCommand struct {
	startedAt time.Time
}

// NewPingCommand creates the command. It is constructed during startup, so
// its creation time doubles as the bot's start time for uptime reporting.
func NewPingCommand() *PingCommand {
	return &PingCommand{startedAt: time.Now()}
}

func (c *PingCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Check whether the bot is alive",
	}
}

func (c *PingCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	content := fmt.Sprintf("Pong! (version: %s, uptime: %s, gateway latency: %s)",
		version.Get(), formatUptime(time.Since(c.startedAt)), s.HeartbeatLatency())

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

// formatUptime renders d as e.g. "2d 3h 4m", "5m 12s" or "8s", dropping
// leading zero units and seconds once the uptime reaches an hour.
func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d / (24 * time.Hour))
	hours := int(d/time.Hour) % 24
	mins := int(d/time.Minute) % 60
	secs := int(d/time.Second) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if days > 0 || hours > 0 || mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if days == 0 && hours == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}
	return strings.Join(parts, " ")
}
