// Package reminder periodically fetches the schedule and sends reminders
// for presentations that are about to start.
package reminder

import (
	"context"
	"sync"
	"time"

	"github.com/ntacamp/404-tools/discord-bot/internal/config"
	"github.com/ntacamp/404-tools/discord-bot/internal/logging"
	"github.com/ntacamp/404-tools/discord-bot/internal/schedule"
)

// Notifier sends a reminder for a single presentation.
type Notifier interface {
	SendReminder(ctx context.Context, p schedule.Presentation) error
}

// Poller owns the fetch/check/send loop. It keeps no state on disk: the
// last-fetched schedule and the set of already-sent reminders live only in
// memory and are lost on restart. That's intentional (see package README) —
// the anti-spam guarantee comes from never reminding about a presentation
// whose start time has already passed, which holds true even right after a
// restart. The one accepted tradeoff: if the bot restarts while "now" falls
// inside a presentation's [reminderAt, Start) window, that reminder may be
// sent a second time.
type Poller struct {
	cfg      *config.Config
	notifier Notifier

	mu    sync.Mutex
	sched []schedule.Presentation
	sent  map[string]bool
}

func NewPoller(cfg *config.Config, notifier Notifier) *Poller {
	return &Poller{
		cfg:      cfg,
		notifier: notifier,
		sent:     make(map[string]bool),
	}
}

// Schedule returns a snapshot of the last successfully fetched schedule.
func (p *Poller) Schedule() []schedule.Presentation {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]schedule.Presentation, len(p.sched))
	copy(out, p.sched)
	return out
}

// Run blocks, checking for due reminders immediately and then on every
// PollInterval tick, until ctx is canceled.
func (p *Poller) Run(ctx context.Context) {
	p.tick(ctx)

	ticker := time.NewTicker(p.cfg.PollInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	fetched, err := schedule.Fetch(ctx, p.cfg.ScheduleURL, p.cfg.Location)
	if err != nil {
		logging.Error("reminder: failed to fetch schedule: %v (keeping previously loaded schedule)", err)
	} else {
		p.mu.Lock()
		changed := !schedulesEqual(p.sched, fetched)
		p.sched = fetched
		p.mu.Unlock()

		if changed {
			logging.Info("reminder: schedule updated (%d presentations)", len(fetched))
		}
	}

	now := time.Now().In(p.cfg.Location)
	offset := time.Duration(p.cfg.ReminderOffsetMinutes) * time.Minute

	for _, pres := range p.Schedule() {
		reminderAt := pres.Start.Add(-offset)

		if now.Before(reminderAt) {
			continue // not due yet
		}
		if !now.Before(pres.Start) {
			continue // already started/finished — never announce past talks
		}
		if p.sent[pres.ID()] {
			continue // already sent this process's lifetime
		}

		if err := p.notifier.SendReminder(ctx, pres); err != nil {
			logging.Error("reminder: failed to send reminder for %q: %v (will retry next tick)", pres.Title, err)
			continue
		}
		p.sent[pres.ID()] = true
	}
}

// schedulesEqual reports whether a and b contain the same presentations in
// the same order (both are always sorted ascending by start time, so order
// itself is meaningful content, not just incidental).
func schedulesEqual(a, b []schedule.Presentation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Start.Equal(b[i].Start) || a[i].Title != b[i].Title || a[i].Speaker != b[i].Speaker {
			return false
		}
	}
	return true
}
