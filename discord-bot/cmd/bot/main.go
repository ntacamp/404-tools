// Command bot runs the 404 Camp Discord presentation-reminder bot.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ntacamp/404-tools/discord-bot/internal/config"
	"github.com/ntacamp/404-tools/discord-bot/internal/discordbot"
	"github.com/ntacamp/404-tools/discord-bot/internal/reminder"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	registry := discordbot.NewCommandRegistry()

	bot, err := discordbot.New(cfg.BotToken, cfg.ChannelID, cfg.ReminderOffsetMinutes, registry)
	if err != nil {
		log.Fatalf("creating discord bot: %v", err)
	}

	poller := reminder.NewPoller(cfg, bot)

	registry.Register(discordbot.NewUpcomingCommand(poller.Schedule, cfg.Location))
	registry.Register(discordbot.NewPingCommand())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bot.Open(); err != nil {
		log.Fatalf("opening discord session: %v", err)
	}
	defer bot.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller.Run(ctx)
	}()

	log.Printf("bot started; polling every %s", cfg.PollInterval.Duration)
	<-ctx.Done()
	log.Println("shutdown signal received, stopping...")
	wg.Wait()
}
