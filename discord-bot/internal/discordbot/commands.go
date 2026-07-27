package discordbot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Command is the interface every slash command implements. To add a new
// command in the future: implement this interface in a new file, then call
// CommandRegistry.Register with it in cmd/bot/main.go — no other wiring is
// required.
type Command interface {
	Definition() *discordgo.ApplicationCommand
	Handle(s *discordgo.Session, i *discordgo.InteractionCreate)
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
		if cmd, ok := r.commands[i.ApplicationCommandData().Name]; ok {
			cmd.Handle(s, i)
		}
	})

	var created []*discordgo.ApplicationCommand
	for _, c := range r.commands {
		ac, err := s.ApplicationCommandCreate(s.State.User.ID, "", c.Definition())
		if err != nil {
			return nil, fmt.Errorf("registering command %q: %w", c.Definition().Name, err)
		}
		created = append(created, ac)
	}

	cleanup := func() {
		for _, ac := range created {
			_ = s.ApplicationCommandDelete(s.State.User.ID, "", ac.ID)
		}
	}
	return cleanup, nil
}
