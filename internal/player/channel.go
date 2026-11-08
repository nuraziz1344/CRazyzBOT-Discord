package player

import (
	"context"
	"errors"
	"fmt"
	"go-tube/pkg/music"
	"go-tube/pkg/youtube"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrUserNotInVoice       = errors.New("USER_NOT_IN_VOICE")
	ErrUserInDifferentVoice = errors.New("USER_IN_DIFFERENT_VOICE")
)

type PlayerEvent int

const (
	EventTrackStart PlayerEvent = iota
	EventStateUpdate
	EventPlayerStopped
)

// OnPlayerStateChange is invoked when the player starts a new track, updates its state, or stops.
var OnPlayerStateChange func(s *discordgo.Session, gp *GuildPlayer, event PlayerEvent, detail string)

// GuildPlayer manages voice connection, queue, and playback state for a single Discord guild.
type GuildPlayer struct {
	mu                 sync.Mutex
	GuildID            string
	VoiceChannelID     string
	TextChannelID      string
	LastPlayerMsgID    string
	LastPlayerChanID   string
	Queue              *Queue
	session            *discordgo.Session
	dgv                *discordgo.VoiceConnection
	isRunning          bool
	isPaused           bool
	skipRequested      bool
	forceRemoveCurrent bool
	currentCancel      context.CancelFunc
	currentCmd         *youtube.PipelineCmd
}

var (
	playersMu sync.RWMutex
	players   = make(map[string]*GuildPlayer)
)

// GetPlayer returns the active GuildPlayer for a guild, or nil if none exists.
func GetPlayer(guildID string) *GuildPlayer {
	playersMu.RLock()
	defer playersMu.RUnlock()
	return players[guildID]
}

// getOrCreatePlayer returns an existing GuildPlayer or initializes a new one.
func getOrCreatePlayer(guildID, voiceChannelID, textChannelID string, s *discordgo.Session) *GuildPlayer {
	playersMu.Lock()
	defer playersMu.Unlock()

	gp, exists := players[guildID]
	if !exists {
		gp = &GuildPlayer{
			GuildID:        guildID,
			VoiceChannelID: voiceChannelID,
			TextChannelID:  textChannelID,
			Queue:          NewQueue(),
			session:        s,
		}
		players[guildID] = gp
	} else {
		gp.mu.Lock()
		if voiceChannelID != "" && gp.VoiceChannelID == "" {
			gp.VoiceChannelID = voiceChannelID
		}
		if textChannelID != "" {
			gp.TextChannelID = textChannelID
		}
		if s != nil {
			gp.session = s
		}
		gp.mu.Unlock()
	}
	return gp
}

func removePlayer(guildID string) {
	playersMu.Lock()
	delete(players, guildID)
	playersMu.Unlock()
}

// ValidateVoiceAccess ensures the user is in a voice channel, and if the bot is already in a voice channel,
// that the user is in the exact same voice channel.
func ValidateVoiceAccess(s *discordgo.Session, guildID, userID string) (string, error) {
	vc, err := s.State.VoiceState(guildID, userID)
	if err != nil || vc == nil || vc.ChannelID == "" {
		return "", ErrUserNotInVoice
	}

	gp := GetPlayer(guildID)
	if gp != nil {
		botVC := gp.GetVoiceChannelID()
		if botVC != "" && botVC != vc.ChannelID {
			return "", ErrUserInDifferentVoice
		}
	}

	return vc.ChannelID, nil
}

func (gp *GuildPlayer) GetVoiceChannelID() string {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.VoiceChannelID
}

func (gp *GuildPlayer) SetVoiceChannelID(chID string) {
	gp.mu.Lock()
	gp.VoiceChannelID = chID
	gp.mu.Unlock()
}

func (gp *GuildPlayer) SetTextChannelID(chID string) {
	gp.mu.Lock()
	if chID != "" {
		gp.TextChannelID = chID
	}
	gp.mu.Unlock()
}

func (gp *GuildPlayer) GetTextChannelID() string {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.TextChannelID
}

func (gp *GuildPlayer) SetLastPlayerMessage(channelID, messageID string) (oldChanID, oldMsgID string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	oldChanID = gp.LastPlayerChanID
	oldMsgID = gp.LastPlayerMsgID
	gp.LastPlayerChanID = channelID
	gp.LastPlayerMsgID = messageID
	return oldChanID, oldMsgID
}

func (gp *GuildPlayer) GetLastPlayerMessage() (channelID, messageID string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.LastPlayerChanID, gp.LastPlayerMsgID
}

func (gp *GuildPlayer) IsPaused() bool {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.isPaused
}

func (gp *GuildPlayer) IsRunning() bool {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.isRunning
}

// SetPaused sets pause state and returns the updated state.
func (gp *GuildPlayer) SetPaused(paused bool) bool {
	gp.mu.Lock()
	gp.isPaused = paused
	s := gp.session
	gp.mu.Unlock()

	if OnPlayerStateChange != nil && s != nil {
		OnPlayerStateChange(s, gp, EventStateUpdate, "")
	}
	return paused
}

// TogglePause flips the pause state and returns the new state (true = paused).
func (gp *GuildPlayer) TogglePause() bool {
	gp.mu.Lock()
	gp.isPaused = !gp.isPaused
	paused := gp.isPaused
	s := gp.session
	gp.mu.Unlock()

	if OnPlayerStateChange != nil && s != nil {
		OnPlayerStateChange(s, gp, EventStateUpdate, "")
	}
	return paused
}

// JoinVoice connects the bot to the target voice channel if not already connected.
func (gp *GuildPlayer) JoinVoice() error {
	gp.mu.Lock()
	s := gp.session
	gid := gp.GuildID
	cid := gp.VoiceChannelID
	existingDGV := gp.dgv
	gp.mu.Unlock()

	if s == nil || cid == "" {
		return ErrUserNotInVoice
	}

	if existingDGV != nil && existingDGV.Status == discordgo.VoiceConnectionStatusReady {
		return nil
	}

	if existingDGV != nil {
		_ = existingDGV.Disconnect(context.Background())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dgv, err := s.ChannelVoiceJoin(ctx, gid, cid, false, true)
	if err != nil {
		return err
	}

	// Wait until Ready or timeout
	deadline := time.Now().Add(10 * time.Second)
	for dgv.Status != discordgo.VoiceConnectionStatusReady {
		if dgv.Status == discordgo.VoiceConnectionStatusDead || time.Now().After(deadline) {
			_ = dgv.Disconnect(context.Background())
			return errors.New("timeout waiting for voice connection to become ready")
		}
		time.Sleep(50 * time.Millisecond)
	}

	gp.mu.Lock()
	gp.dgv = dgv
	gp.mu.Unlock()
	return nil
}

// EnqueueTracks adds one or more tracks to the guild's queue and starts the player loop if idle.
func EnqueueTracks(s *discordgo.Session, guildID, voiceChannelID, textChannelID string, tracks ...*music.Track) (*GuildPlayer, int, error) {
	if voiceChannelID == "" {
		return nil, 0, ErrUserNotInVoice
	}

	gp := getOrCreatePlayer(guildID, voiceChannelID, textChannelID, s)
	total := gp.Queue.Enqueue(tracks...)

	gp.mu.Lock()
	shouldStart := !gp.isRunning
	if shouldStart {
		gp.isRunning = true
		gp.isPaused = false
	}
	gp.mu.Unlock()

	if shouldStart {
		go gp.runPlayerLoop()
	} else {
		// Trigger background prefetch if the newly added track is at index 1 (up next)
		go gp.prefetchNextTrack()
	}

	return gp, total, nil
}

// Skip stops the currently playing track and advances to the next one in queue.
func Skip(guildID string) bool {
	gp := GetPlayer(guildID)
	if gp == nil || gp.Queue.Len() == 0 {
		return false
	}

	gp.mu.Lock()
	gp.skipRequested = true
	gp.isPaused = false
	cancel := gp.currentCancel
	cmd := gp.currentCmd
	gp.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cmd != nil {
		_ = cmd.Kill()
	}
	return true
}

// RemoveFromQueue deletes 1 track at 1-based position `index`.
// If index == 1 (currently playing track), it skips and forces removal of index 1.
func RemoveFromQueue(guildID string, oneBasedIndex int) (*music.Track, error) {
	gp := GetPlayer(guildID)
	if gp == nil {
		return nil, ErrQueueEmpty
	}

	removed, wasPlaying, err := gp.Queue.Remove(oneBasedIndex)
	if err != nil {
		return nil, err
	}

	if wasPlaying {
		gp.mu.Lock()
		gp.skipRequested = true
		gp.forceRemoveCurrent = true
		gp.isPaused = false
		cancel := gp.currentCancel
		cmd := gp.currentCmd
		gp.mu.Unlock()

		if cancel != nil {
			cancel()
		}
		if cmd != nil {
			_ = cmd.Kill()
		}
	} else {
		go gp.prefetchNextTrack()
		gp.mu.Lock()
		s := gp.session
		gp.mu.Unlock()
		if OnPlayerStateChange != nil && s != nil {
			OnPlayerStateChange(s, gp, EventStateUpdate, "")
		}
	}

	return removed, nil
}

// Clear stops playback, clears the queue, and disconnects from the voice channel.
func Clear(guildID string) {
	ClearAndLeave(guildID, "")
}

// ClearAndLeave stops playback, clears the queue, disconnects from voice, and notifies UI with optional reason.
func ClearAndLeave(guildID string, reason string) {
	gp := GetPlayer(guildID)
	if gp == nil {
		return
	}

	gp.Queue.Clear()

	gp.mu.Lock()
	gp.isRunning = false
	gp.isPaused = false
	cancel := gp.currentCancel
	cmd := gp.currentCmd
	dgv := gp.dgv
	gp.dgv = nil
	gp.VoiceChannelID = ""
	s := gp.session
	gp.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cmd != nil {
		_ = cmd.Kill()
	}
	if dgv != nil {
		_ = dgv.Disconnect(context.Background())
	}

	removePlayer(guildID)

	if OnPlayerStateChange != nil && s != nil {
		OnPlayerStateChange(s, gp, EventPlayerStopped, reason)
	}
}

// ClearAll disconnects all active guild players on bot shutdown.
func ClearAll() {
	playersMu.RLock()
	gids := make([]string, 0, len(players))
	for gid := range players {
		gids = append(gids, gid)
	}
	playersMu.RUnlock()

	for _, gid := range gids {
		ClearAndLeave(gid, "")
	}
}

// HandleVoiceStateUpdate monitors voice state changes to auto-leave when the voice channel becomes empty
// (only the bot remains) or when the bot itself is disconnected.
func HandleVoiceStateUpdate(s *discordgo.Session, vsu *discordgo.VoiceStateUpdate, botID string) {
	if vsu == nil || vsu.GuildID == "" {
		return
	}

	gp := GetPlayer(vsu.GuildID)
	if gp == nil {
		return
	}

	// If the bot itself was disconnected or moved
	if vsu.UserID == botID {
		if vsu.ChannelID == "" {
			if Debug {
				fmt.Printf("[Voice] Bot was disconnected from guild %s\n", vsu.GuildID)
			}
			ClearAndLeave(vsu.GuildID, "")
			return
		}
		gp.SetVoiceChannelID(vsu.ChannelID)
	}

	botChannelID := gp.GetVoiceChannelID()
	if botChannelID == "" {
		return
	}

	guild, err := s.State.Guild(vsu.GuildID)
	if err != nil || guild == nil {
		return
	}

	// Count non-bot users remaining in the bot's voice channel
	humanCount := 0
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != botChannelID || vs.UserID == botID {
			continue
		}
		isBot := false
		if member, mErr := s.State.Member(vsu.GuildID, vs.UserID); mErr == nil && member != nil && member.User != nil {
			isBot = member.User.Bot
		}
		if !isBot {
			humanCount++
		}
	}

	if humanCount == 0 {
		if Debug {
			fmt.Printf("[Voice] Voice channel %s is empty (only bot left), leaving...\n", botChannelID)
		}
		ClearAndLeave(vsu.GuildID, "Voice channel kosong, bot otomatis keluar.")
	}
}
