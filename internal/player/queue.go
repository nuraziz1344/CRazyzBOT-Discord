package player

import (
	"errors"
	"go-tube/pkg/music"
	"math/rand/v2"
	"strings"
	"sync"
)

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

func (r RepeatMode) String() string {
	switch r {
	case RepeatAll:
		return "All"
	case RepeatOne:
		return "1"
	default:
		return "Off"
	}
}

func ParseRepeatMode(arg string) (RepeatMode, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "off", "none", "0", "false":
		return RepeatOff, true
	case "all", "queue", "loop":
		return RepeatAll, true
	case "1", "one", "single", "song", "track":
		return RepeatOne, true
	default:
		return RepeatOff, false
	}
}

var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrInvalidQueueIndex = errors.New("invalid queue index")
)

// Queue is a thread-safe music queue supporting Repeat (Off, All, 1), Shuffle, and item deletion.
type Queue struct {
	mu     sync.RWMutex
	items  []*music.Track
	repeat RepeatMode
}

func NewQueue() *Queue {
	return &Queue{
		items:  make([]*music.Track, 0),
		repeat: RepeatOff,
	}
}

// Enqueue appends one or more tracks to the queue and returns the new queue length.
func (q *Queue) Enqueue(tracks ...*music.Track) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, t := range tracks {
		if t != nil {
			q.items = append(q.items, t.Clone())
		}
	}
	return len(q.items)
}

// Current returns the currently playing track (index 0), or nil if empty.
func (q *Queue) Current() *music.Track {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.items) == 0 {
		return nil
	}
	return q.items[0]
}

// PeekNext returns the next track to be played after the current one, for background prefetching.
func (q *Queue) PeekNext() *music.Track {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.items) > 1 {
		return q.items[1]
	}
	if len(q.items) == 1 && (q.repeat == RepeatOne || q.repeat == RepeatAll) {
		return q.items[0]
	}
	return nil
}

// Advance transitions the queue after a track finishes.
// If forceRemoveCurrent is true (e.g. user deleted track #1), index 0 is removed regardless of repeat mode.
// If skipped is true and RepeatOne is active, it advances to the next track instead of repeating the skipped track.
func (q *Queue) Advance(skipped bool, forceRemoveCurrent bool) *music.Track {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	if forceRemoveCurrent {
		q.items = q.items[1:]
		if len(q.items) == 0 {
			return nil
		}
		return q.items[0]
	}

	switch q.repeat {
	case RepeatOne:
		if skipped && len(q.items) > 1 {
			q.items = q.items[1:]
		}
		return q.items[0]

	case RepeatAll:
		if len(q.items) > 1 {
			first := q.items[0]
			q.items = append(q.items[1:], first)
		}
		return q.items[0]

	default: // RepeatOff
		q.items = q.items[1:]
		if len(q.items) == 0 {
			return nil
		}
		return q.items[0]
	}
}

// Remove deletes a single track by its 1-based position in the queue.
// Returns the removed track and a boolean indicating whether it was the currently playing track (index 1).
func (q *Queue) Remove(oneBasedIndex int) (removed *music.Track, wasPlaying bool, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, false, ErrQueueEmpty
	}
	idx := oneBasedIndex - 1
	if idx < 0 || idx >= len(q.items) {
		return nil, false, ErrInvalidQueueIndex
	}

	removed = q.items[idx]
	if idx == 0 {
		// Caller will trigger Skip with forceRemoveCurrent=true so the active stream stops cleanly
		return removed, true, nil
	}

	q.items = append(q.items[:idx], q.items[idx+1:]...)
	return removed, false, nil
}

// Shuffle randomizes all upcoming tracks (items[1:]) while keeping items[0] (currently playing) intact.
func (q *Queue) Shuffle() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) <= 2 {
		if len(q.items) <= 1 {
			return 0
		}
		return 1
	}

	upcoming := q.items[1:]
	rand.Shuffle(len(upcoming), func(i, j int) {
		upcoming[i], upcoming[j] = upcoming[j], upcoming[i]
	})
	return len(upcoming)
}

// Clear removes all tracks from the queue.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
}

// Snapshot returns a copy of all tracks in the queue and the current repeat mode.
func (q *Queue) Snapshot() ([]*music.Track, RepeatMode) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	cp := make([]*music.Track, len(q.items))
	copy(cp, q.items)
	return cp, q.repeat
}

// Len returns the number of tracks in the queue.
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

// Repeat returns the current repeat mode.
func (q *Queue) Repeat() RepeatMode {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.repeat
}

// SetRepeat sets the repeat mode explicitly.
func (q *Queue) SetRepeat(mode RepeatMode) RepeatMode {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.repeat = mode
	return q.repeat
}

// CycleRepeat cycles RepeatOff -> RepeatAll -> RepeatOne -> RepeatOff.
func (q *Queue) CycleRepeat() RepeatMode {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch q.repeat {
	case RepeatOff:
		q.repeat = RepeatAll
	case RepeatAll:
		q.repeat = RepeatOne
	default:
		q.repeat = RepeatOff
	}
	return q.repeat
}
