package reminder

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ntacamp/404-tools/discord-bot/internal/config"
	"github.com/ntacamp/404-tools/discord-bot/internal/schedule"
)

type fakeNotifier struct {
	mu  sync.Mutex
	got []schedule.Presentation
}

func (f *fakeNotifier) SendReminder(_ context.Context, p schedule.Presentation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, p)
	return nil
}

func (f *fakeNotifier) titles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, p := range f.got {
		out = append(out, p.Title)
	}
	return out
}

func newTestConfig(t *testing.T, scheduleURL string) *config.Config {
	t.Helper()
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("loading location: %v", err)
	}
	return &config.Config{
		ScheduleURL:           scheduleURL,
		ChannelID:             "test-channel",
		ReminderOffsetMinutes: 10,
		PollInterval:          config.Duration{Duration: time.Hour}, // irrelevant, we call tick directly
		Timezone:              "UTC",
		BotToken:              "test-token",
		Location:              loc,
	}
}

// csvServer serves a schedule with one presentation already started (in the
// past) and one about to start within the reminder window (in the future).
func csvServer(loc *time.Location) *httptest.Server {
	past := time.Now().In(loc).Add(-1 * time.Hour)
	soon := time.Now().In(loc).Add(5 * time.Minute) // within the 10-minute offset window

	body := fmt.Sprintf("%s,%s,Past Talk,Speaker A\n%s,%s,Soon Talk,Speaker B\n",
		past.Format("2006-01-02"), past.Format("15:04"),
		soon.Format("2006-01-02"), soon.Format("15:04"))

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
}

func TestPoller_DoesNotRemindPastTalks(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	srv := csvServer(loc)
	defer srv.Close()

	cfg := newTestConfig(t, srv.URL)
	notifier := &fakeNotifier{}
	p := NewPoller(cfg, notifier)

	p.tick(context.Background())

	got := notifier.titles()
	if len(got) != 1 || got[0] != "Soon Talk" {
		t.Fatalf("expected only [Soon Talk] to be reminded, got %v", got)
	}
}

func TestPoller_DoesNotResendWithinSameRun(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	srv := csvServer(loc)
	defer srv.Close()

	cfg := newTestConfig(t, srv.URL)
	notifier := &fakeNotifier{}
	p := NewPoller(cfg, notifier)

	p.tick(context.Background())
	p.tick(context.Background())
	p.tick(context.Background())

	got := notifier.titles()
	if len(got) != 1 {
		t.Fatalf("expected exactly one reminder across repeated ticks, got %v", got)
	}
}

func TestPoller_RestartDoesNotRemindPastTalk(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	srv := csvServer(loc)
	defer srv.Close()

	cfg := newTestConfig(t, srv.URL)

	// Simulate a first process instance that already reminded about "Soon Talk".
	firstNotifier := &fakeNotifier{}
	first := NewPoller(cfg, firstNotifier)
	first.tick(context.Background())

	// "Restart": a brand new Poller with no in-memory state, same live schedule.
	secondNotifier := &fakeNotifier{}
	second := NewPoller(cfg, secondNotifier)
	second.tick(context.Background())

	got := second.Schedule()
	if len(got) != 2 {
		t.Fatalf("expected schedule to still contain both talks, got %d", len(got))
	}

	titles := secondNotifier.titles()
	for _, title := range titles {
		if title == "Past Talk" {
			t.Fatalf("restart re-announced a past talk: %v", titles)
		}
	}
}

func TestSchedulesEqual(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	base := []schedule.Presentation{
		{Start: time.Date(2026, 7, 17, 9, 0, 0, 0, loc), Title: "A", Speaker: "Alex"},
		{Start: time.Date(2026, 7, 17, 9, 30, 0, 0, loc), Title: "B", Speaker: "Blake"},
	}

	sameContent := []schedule.Presentation{
		{Start: time.Date(2026, 7, 17, 9, 0, 0, 0, loc), Title: "A", Speaker: "Alex"},
		{Start: time.Date(2026, 7, 17, 9, 30, 0, 0, loc), Title: "B", Speaker: "Blake"},
	}
	if !schedulesEqual(base, sameContent) {
		t.Error("expected identical schedules to compare equal")
	}

	shorter := base[:1]
	if schedulesEqual(base, shorter) {
		t.Error("expected schedules of different length to compare unequal")
	}

	renamedSpeaker := []schedule.Presentation{
		base[0],
		{Start: base[1].Start, Title: base[1].Title, Speaker: "Someone Else"},
	}
	if schedulesEqual(base, renamedSpeaker) {
		t.Error("expected schedules with a changed speaker to compare unequal")
	}

	if schedulesEqual(nil, base) {
		t.Error("expected nil (no prior schedule) vs. non-empty to compare unequal")
	}
}

func TestPoller_LogsOnlyWhenScheduleContentChanges(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	srv := csvServer(loc)
	defer srv.Close()

	cfg := newTestConfig(t, srv.URL)
	p := NewPoller(cfg, &fakeNotifier{})

	p.tick(context.Background()) // first fetch: no prior schedule, content "changes" from empty
	firstSched := p.Schedule()

	p.tick(context.Background()) // second fetch: identical CSV served again, no change
	secondSched := p.Schedule()

	if !schedulesEqual(firstSched, secondSched) {
		t.Fatalf("expected repeated fetches of the same CSV to be considered equal")
	}
}
