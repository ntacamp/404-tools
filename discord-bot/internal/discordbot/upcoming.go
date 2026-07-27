package discordbot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/ntacamp/404-tools/discord-bot/internal/schedule"
)

// maxUpcomingListed is how many presentations /upcoming shows at most.
const maxUpcomingListed = 5

// UpcomingCommand implements "/upcoming", listing the next few presentations
// that haven't started yet. It depends only on the schedule package (via a
// plain function value for fetching the current schedule) to avoid coupling
// discordbot to the reminder package.
type UpcomingCommand struct {
	getSchedule func() []schedule.Presentation
	loc         *time.Location
}

func NewUpcomingCommand(getSchedule func() []schedule.Presentation, loc *time.Location) *UpcomingCommand {
	return &UpcomingCommand{getSchedule: getSchedule, loc: loc}
}

func (c *UpcomingCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "upcoming",
		Description: "Show the next few upcoming presentations",
	}
}

func (c *UpcomingCommand) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	now := time.Now().In(c.loc)

	var lines []string
	for _, p := range c.getSchedule() {
		if !p.Start.After(now) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s — **%s** (%s)", formatStart(now, p.Start), p.Title, p.Speaker))
		if len(lines) == maxUpcomingListed {
			break
		}
	}

	content := "No more presentations scheduled."
	if len(lines) > 0 {
		content = strings.Join(lines, "\n")
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

// formatStart renders start as just a time if it falls on the same calendar
// day as now, or with a short date prefix otherwise.
func formatStart(now, start time.Time) string {
	ny, nm, nd := now.Date()
	sy, sm, sd := start.Date()
	if ny == sy && nm == sm && nd == sd {
		return start.Format("15:04")
	}
	return start.Format("Jan 2, 15:04")
}
