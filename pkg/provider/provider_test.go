package provider

import (
	"go-tube/pkg/music"
	"testing"
	"time"
)

func TestNormalizeTitleKey(t *testing.T) {
	a := NormalizeTitleKey("Rick Astley", "Rick Astley - Never Gonna Give You Up (Official Video)")
	b := NormalizeTitleKey("Rick Astley", "Never Gonna Give You Up (Lyrics)")
	c := NormalizeTitleKey("Rick Astley", "Never Gonna Give You Up")
	d := NormalizeTitleKey("Rick Astley", "Never Gonna Give You Up [4K Remastered]")

	if a == "" {
		t.Fatal("expected non-empty normalized key")
	}
	if a != b || b != c || c != d {
		t.Fatalf("expected identical normalized keys, got a=%q b=%q c=%q d=%q", a, b, c, d)
	}
}

func TestSelectRelatedCandidate(t *testing.T) {
	seedID := "seed123"
	seedNorm := NormalizeTitleKey("Artist A", "Seed Song")

	candidates := []*music.Track{
		{ID: "seed123", Title: "Seed Song", Artist: "Artist A", Duration: 3 * time.Minute},
		{ID: "hist1", Title: "History Song", Artist: "Artist B", Duration: 3 * time.Minute},
		{ID: "long1", Title: "Long Mix", Artist: "Artist C", Duration: 45 * time.Minute},
		{ID: "good1", Title: "Good Related", Artist: "Artist D", Duration: 4 * time.Minute},
	}
	excludeIDs := map[string]bool{"hist1": true}
	excludeTitles := map[string]bool{}

	// Pass 1: skips seed, history, and >15m track -> picks good1
	got := selectRelatedCandidate(candidates, seedID, seedNorm, excludeIDs, excludeTitles)
	if got == nil || got.ID != "good1" {
		t.Fatalf("expected good1, got %+v", got)
	}

	// Pass 2: if good1 is excluded by title, relaxes duration cap -> picks long1
	excludeTitles[NormalizeTitleKey("Artist D", "Good Related")] = true
	got = selectRelatedCandidate(candidates, seedID, seedNorm, excludeIDs, excludeTitles)
	if got == nil || got.ID != "long1" {
		t.Fatalf("expected long1 when good1 excluded, got %+v", got)
	}

	// Pass 3: if all non-seed candidates are in history, falls back to non-seed candidate
	excludeIDs["long1"] = true
	got = selectRelatedCandidate(candidates, seedID, seedNorm, excludeIDs, excludeTitles)
	if got == nil || got.ID == seedID {
		t.Fatalf("expected fallback non-seed candidate, got %+v", got)
	}
}
