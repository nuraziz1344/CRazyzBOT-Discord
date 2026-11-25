package player

import (
	"go-tube/pkg/music"
	"testing"
)

func makeTrack(id, title string) *music.Track {
	return &music.Track{
		ID:     id,
		Title:  title,
		Source: music.SourceYouTube,
	}
}

func TestQueueEnqueueAndAdvanceRepeatOff(t *testing.T) {
	q := NewQueue()
	n := q.Enqueue(makeTrack("1", "Song 1"), makeTrack("2", "Song 2"), makeTrack("3", "Song 3"))
	if n != 3 {
		t.Fatalf("expected length 3, got %d", n)
	}

	if cur := q.Current(); cur == nil || cur.ID != "1" {
		t.Fatalf("expected current track 1, got %v", cur)
	}

	// Natural advance in RepeatOff removes Song 1
	next := q.Advance(false, false)
	if next == nil || next.ID != "2" {
		t.Fatalf("expected next track 2, got %v", next)
	}
	if q.Len() != 2 {
		t.Fatalf("expected length 2, got %d", q.Len())
	}

	next = q.Advance(false, false)
	if next == nil || next.ID != "3" {
		t.Fatalf("expected next track 3, got %v", next)
	}

	next = q.Advance(false, false)
	if next != nil {
		t.Fatalf("expected nil when queue finishes in RepeatOff, got %v", next)
	}
}

func TestQueueRepeatOneAndRepeatAll(t *testing.T) {
	q := NewQueue()
	q.Enqueue(makeTrack("1", "Song 1"), makeTrack("2", "Song 2"))

	// RepeatOne: natural finish replays Song 1
	q.SetRepeat(RepeatOne)
	next := q.Advance(false, false)
	if next == nil || next.ID != "1" {
		t.Fatalf("expected RepeatOne to keep Song 1 on natural finish, got %v", next)
	}

	// RepeatOne: manual skip advances to Song 2
	next = q.Advance(true, false)
	if next == nil || next.ID != "2" {
		t.Fatalf("expected RepeatOne manual skip to advance to Song 2, got %v", next)
	}

	// RepeatAll: rotates tracks
	q.Clear()
	q.Enqueue(makeTrack("A", "Song A"), makeTrack("B", "Song B"))
	q.SetRepeat(RepeatAll)

	next = q.Advance(false, false)
	if next == nil || next.ID != "B" {
		t.Fatalf("expected RepeatAll to advance to Song B, got %v", next)
	}
	if q.Len() != 2 {
		t.Fatalf("expected RepeatAll to preserve queue length 2, got %d", q.Len())
	}

	next = q.Advance(false, false)
	if next == nil || next.ID != "A" {
		t.Fatalf("expected RepeatAll to wrap back to Song A, got %v", next)
	}
}

func TestQueueRemoveAndShuffle(t *testing.T) {
	q := NewQueue()
	q.Enqueue(
		makeTrack("1", "Song 1"),
		makeTrack("2", "Song 2"),
		makeTrack("3", "Song 3"),
		makeTrack("4", "Song 4"),
	)

	// Remove item #3 ("Song 3")
	removed, wasPlaying, err := q.Remove(3)
	if err != nil {
		t.Fatalf("unexpected error removing item 3: %v", err)
	}
	if wasPlaying {
		t.Fatalf("expected wasPlaying=false for item 3")
	}
	if removed.ID != "3" {
		t.Fatalf("expected removed ID '3', got %s", removed.ID)
	}
	if q.Len() != 3 {
		t.Fatalf("expected remaining length 3, got %d", q.Len())
	}

	// Shuffle keeps index 0 ("Song 1") intact
	shuffledCount := q.Shuffle()
	if shuffledCount != 2 {
		t.Fatalf("expected 2 upcoming tracks shuffled, got %d", shuffledCount)
	}
	if q.Current().ID != "1" {
		t.Fatalf("expected currently playing track '1' to stay at index 0 after shuffle, got %s", q.Current().ID)
	}
}
