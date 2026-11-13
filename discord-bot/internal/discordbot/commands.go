package discordbot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/ntacamp/404-tools/discord-bot/internal/logging"
)

// Command is the interface every slash command implements. To add a new
// command in the future: implement this interface in a new file, then call
// CommandRegistry.Register with it in cmd/bot/main.go — no other wiring is
// required.
type Command interface {
	Definition() *discordgo.ApplicationCommand
	// Handle responds to the interaction. A returned error is logged by the
	// registry; commands don't need to log invocations themselves.
	Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error
}

// CommandRegistry holds the set of slash commands the bot exposes.
type CommandRegistry struct {
	commands map[string]Command
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{commands: make(map[string]Command)}
}

// Register adds a command. Call this before Bind (i.e. before Bot.Open).
func (r *CommandRegistry) Register(c Command) {
	r.commands[c.Definition().Name] = c
}

// Bind wires up interaction dispatch and registers all commands globally
// with Discord. It returns a cleanup function that deletes them again,
// intended to be called on shutdown.
//
// Commands are registered globally (empty guild ID) since there is no
// per-guild config. Global command registration/updates can take up to
// roughly an hour to propagate on Discord's side — this is a Discord API
// characteristic, not a bug here.
func (r *CommandRegistry) Bind(s *discordgo.Session) (func(), error) {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		name := i.ApplicationCommandData().Name
		who := describeInvoker(i)

		cmd, ok := r.commands[name]
		if !ok {
			logging.Error("command: unknown /%s from %s", name, who)
			return
		}

		logging.Info("command: /%s invoked by %s", name, who)
		start := time.Now()
		if err := cmd.Handle(s, i); err != nil {
			logging.Error("command: /%s from %s failed after %s: %v", name, who, time.Since(start).Round(time.Millisecond), err)
			return
		}
		logging.Info("command: /%s from %s handled in %s", name, who, time.Since(start).Round(time.Millisecond))
	})

	var created []*discordgo.ApplicationCommand
	for _, c := range r.commands {
		ac, err := s.ApplicationCommandCreate(s.State.User.ID, "", c.Definition())
		if err != nil {
			return nil, fmt.Errorf("registering command %q: %w", c.Definition().Name, err)
		}
		created = append(created, ac)
		logging.Info("command: registered /%s", ac.Name)
	}

	cleanup := func() {
		for _, ac := range created {
			if err := s.ApplicationCommandDelete(s.State.User.ID, "", ac.ID); err != nil {
				logging.Error("command: failed to deregister /%s: %v", ac.Name, err)
				continue
			}
			logging.Info("command: deregistered /%s", ac.Name)
		}
	}
	return cleanup, nil
}

// describeInvoker renders who ran an interaction and where, e.g.
// `user "alice" (123) in guild 456 channel 789`. Guild interactions carry the
// user in Member; DMs carry it in User.
func describeInvoker(i *discordgo.InteractionCreate) string {
	u := i.User
	if i.Member != nil && i.Member.User != nil {
		u = i.Member.User
	}

	who := "unknown user"
	if u != nil {
		who = fmt.Sprintf("user %q (%s)", u.Username, u.ID)
	}

	if i.GuildID == "" {
		return fmt.Sprintf("%s in DM channel %s", who, i.ChannelID)
	}
	return fmt.Sprintf("%s in guild %s channel %s", who, i.GuildID, i.ChannelID)
}
