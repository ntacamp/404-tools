// Package schedule fetches and parses the conference schedule CSV.
package schedule

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"
)

// Presentation is a single scheduled talk.
type Presentation struct {
	Start   time.Time
	Title   string
	Speaker string
}

// ID is a stable key used to dedup reminders for this presentation within a
// single process run. There is no explicit ID column in the CSV, so start
// time + title is used instead.
func (p Presentation) ID() string {
	return p.Start.Format(time.RFC3339) + "|" + p.Title
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// IsURL reports whether source should be treated as an HTTP(S) URL rather
// than a local file path.
func IsURL(source string) bool {
	u, err := url.Parse(source)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// Fetch loads and parses the schedule CSV from source, which is either an
// http(s) URL or a local file path (as determined by IsURL).
func Fetch(ctx context.Context, source string, loc *time.Location) ([]Presentation, error) {
	if IsURL(source) {
		return fetchURL(ctx, source, loc)
	}
	return fetchFile(source, loc)
}

func fetchURL(ctx context.Context, url string, loc *time.Location) ([]Presentation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching schedule: unexpected status %s", resp.Status)
	}

	return Parse(resp.Body, loc)
}

func fetchFile(path string, loc *time.Location) ([]Presentation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening schedule file: %w", err)
	}
	defer f.Close()

	return Parse(f, loc)
}

// Parse reads presentations from CSV data shaped like:
//
//	2026-07-17,09:00,Welcome and opening remarks,Alex Chen
//
// with no header row. Times are interpreted in loc.
func Parse(r io.Reader, loc *time.Location) ([]Presentation, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 4

	var out []Presentation
	for lineNo := 1; ; lineNo++ {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing schedule csv (record %d): %w", lineNo, err)
		}

		start, err := time.ParseInLocation("2006-01-02 15:04", record[0]+" "+record[1], loc)
		if err != nil {
			return nil, fmt.Errorf("parsing date/time on record %d (%q %q): %w", lineNo, record[0], record[1], err)
		}

		out = append(out, Presentation{
			Start:   start,
			Title:   record[2],
			Speaker: record[3],
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}
