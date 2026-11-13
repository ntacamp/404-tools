// Package discordbot wraps the Discord session: sending reminders and
// dispatching slash commands via a CommandRegistry.
package discordbot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/ntacamp/404-tools/discord-bot/internal/logging"
	"github.com/ntacamp/404-tools/discord-bot/internal/schedule"
)

// Bot owns the Discord session and posts reminders to a single channel.
type Bot struct {
	session       *discordgo.Session
	channelID     string
	offsetMinutes int
	registry      *CommandRegistry
	cleanup       func()
}

// New creates a Bot. It does not connect to Discord yet — call Open for that.
func New(token, channelID string, offsetMinutes int, registry *CommandRegistry) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("creating discord session: %w", err)
	}

	// The bot only sends channel messages and handles slash-command
	// interactions, so no privileged intents (message content, members,
	// presence) are needed.
	session.Identify.Intents = discordgo.IntentsGuilds

	return &Bot{
		session:       session,
		channelID:     channelID,
		offsetMinutes: offsetMinutes,
		registry:      registry,
	}, nil
}

// Open connects to Discord and registers slash commands from the registry.
func (b *Bot) Open() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("opening discord session: %w", err)
	}
	logging.Info("discord: connected as %s (%s)", b.session.State.User.Username, b.session.State.User.ID)

	cleanup, err := b.registry.Bind(b.session)
	if err != nil {
		b.session.Close()
		return fmt.Errorf("registering commands: %w", err)
	}
	b.cleanup = cleanup
	logging.Info("discord: registered %d slash command(s)", len(b.registry.commands))

	return nil
}

// Close deregisters slash commands and closes the Discord session.
func (b *Bot) Close() error {
	if b.cleanup != nil {
		b.cleanup()
	}
	err := b.session.Close()
	if err == nil {
		logging.Info("discord: session closed")
	}
	return err
}

// SendReminder posts a plain reminder message for p to the configured channel.
func (b *Bot) SendReminder(_ context.Context, p schedule.Presentation) error {
	text := fmt.Sprintf("Starting in %d minutes: **%s** at %s — %s",
		b.offsetMinutes, p.Title, p.Start.Format("15:04"), p.Speaker)

	_, err := b.session.ChannelMessageSend(b.channelID, text)
	if err != nil {
		return fmt.Errorf("sending reminder message: %w", err)
	}
	logging.Info("discord: reminder sent to channel %s for %q (%s, starts %s)",
		b.channelID, p.Title, p.Speaker, p.Start.Format("2006-01-02 15:04"))
	return nil
}
