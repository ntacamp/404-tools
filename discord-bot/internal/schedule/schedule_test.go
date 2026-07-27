package schedule

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParse_ExampleCSV(t *testing.T) {
	f, err := os.Open("../../presentations.csv.example")
	if err != nil {
		t.Fatalf("opening example csv: %v", err)
	}
	defer f.Close()

	loc, err := time.LoadLocation("Europe/Vilnius")
	if err != nil {
		t.Fatalf("loading location: %v", err)
	}

	got, err := Parse(f, loc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 presentations, got %d: %+v", len(got), got)
	}

	want := Presentation{
		Start:   time.Date(2026, 7, 17, 9, 0, 0, 0, loc),
		Title:   "Welcome and opening remarks",
		Speaker: "Alex Chen",
	}
	if !got[0].Start.Equal(want.Start) || got[0].Title != want.Title || got[0].Speaker != want.Speaker {
		t.Errorf("first presentation = %+v, want %+v", got[0], want)
	}

	for i := 1; i < len(got); i++ {
		if got[i].Start.Before(got[i-1].Start) {
			t.Errorf("presentations not sorted ascending: %+v before %+v", got[i], got[i-1])
		}
	}
}

func TestIsURL(t *testing.T) {
	cases := map[string]bool{
		"https://404.notrollsallowed.com/schedule.csv": true,
		"http://example.com/schedule.csv":              true,
		"./presentations.csv":                          false,
		"presentations.csv":                            false,
		"/etc/schedule.csv":                            false,
		"C:\\schedule.csv":                             false,
	}
	for in, want := range cases {
		if got := IsURL(in); got != want {
			t.Errorf("IsURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFetch_LocalFile(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Vilnius")
	if err != nil {
		t.Fatalf("loading location: %v", err)
	}

	got, err := Fetch(context.Background(), "../../presentations.csv.example", loc)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 presentations, got %d: %+v", len(got), got)
	}
}
