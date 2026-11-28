package commands

import (
	"go-tube/internal/player"
	"go-tube/pkg/music"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestBuildPlayerComponentsAutoplayButton(t *testing.T) {
	if got := BuildPlayerComponents(nil); len(got) != 0 {
		t.Fatalf("expected empty components for nil player, got %d", len(got))
	}

	gp := &player.GuildPlayer{
		Queue: player.NewQueueWithAutoplay(false),
	}
	gp.Queue.Enqueue(&music.Track{Title: "Song A"})
	if got := BuildPlayerComponents(gp); len(got) != 0 {
		t.Fatalf("expected empty components for non-running player, got %d", len(got))
	}

	offRows := buildPlayerComponents(false, player.RepeatOff, false)
	if len(offRows) != 2 {
		t.Fatalf("expected 2 ActionsRows, got %d", len(offRows))
	}

	row1, ok := offRows[0].(discordgo.ActionsRow)
	if !ok || len(row1.Components) != 5 {
		t.Fatalf("expected row 1 to be ActionsRow with 5 buttons, got %#v", offRows[0])
	}

	row2Off, ok := offRows[1].(discordgo.ActionsRow)
	if !ok || len(row2Off.Components) != 1 {
		t.Fatalf("expected row 2 to be ActionsRow with 1 button, got %#v", offRows[1])
	}
	btnOff, ok := row2Off.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected discordgo.Button in row 2, got %T", row2Off.Components[0])
	}
	if btnOff.CustomID != BtnAutoplay || btnOff.Label != "Autoplay: Off" || btnOff.Style != discordgo.SecondaryButton {
		t.Errorf("unexpected Off button: %+v", btnOff)
	}

	onRows := buildPlayerComponents(false, player.RepeatOff, true)
	if len(onRows) != 2 {
		t.Fatalf("expected 2 ActionsRows when autoplay is on, got %d", len(onRows))
	}
	row2On := onRows[1].(discordgo.ActionsRow)
	btnOn := row2On.Components[0].(discordgo.Button)
	if btnOn.CustomID != BtnAutoplay || btnOn.Label != "Autoplay: On" || btnOn.Style != discordgo.SuccessButton {
		t.Errorf("unexpected On button: %+v", btnOn)
	}
}

func TestBuildNowPlayingEmbedAutoplayFieldAndRequester(t *testing.T) {
	t.Run("user requested track", func(t *testing.T) {
		gp := &player.GuildPlayer{
			Queue: player.NewQueueWithAutoplay(false),
		}
		gp.Queue.Enqueue(&music.Track{
			Title:       "Manual Song",
			Source:      music.SourceYouTube,
			Duration:    2 * time.Minute,
			RequestedBy: "123456789",
		})

		embed := BuildNowPlayingEmbed(gp, "")
		if embed == nil {
			t.Fatal("expected non-nil embed")
		}

		var autoplayVal, requesterVal string
		for _, f := range embed.Fields {
			if f.Name == "Autoplay" {
				autoplayVal = f.Value
			}
			if f.Name == "Diminta Oleh" {
				requesterVal = f.Value
			}
		}
		if autoplayVal != "Off" {
			t.Errorf("expected Autoplay field 'Off', got %q", autoplayVal)
		}
		if requesterVal != "<@123456789>" {
			t.Errorf("expected Diminta Oleh '<@123456789>', got %q", requesterVal)
		}
	})

	t.Run("autoplay track", func(t *testing.T) {
		gp := &player.GuildPlayer{
			Queue: player.NewQueueWithAutoplay(true),
		}
		gp.Queue.Enqueue(&music.Track{
			Title:    "Related Song",
			Source:   music.SourceYouTube,
			Duration: 3 * time.Minute,
			Autoplay: true,
		})

		embed := BuildNowPlayingEmbed(gp, "")
		if embed == nil {
			t.Fatal("expected non-nil embed")
		}

		var autoplayVal, requesterVal string
		for _, f := range embed.Fields {
			if f.Name == "Autoplay" {
				autoplayVal = f.Value
			}
			if f.Name == "Diminta Oleh" {
				requesterVal = f.Value
			}
		}
		if autoplayVal != "On" {
			t.Errorf("expected Autoplay field 'On', got %q", autoplayVal)
		}
		if requesterVal != "📻 Autoplay" {
			t.Errorf("expected Diminta Oleh '📻 Autoplay', got %q", requesterVal)
		}
	})
}

func TestFormatTrackListAutoplay(t *testing.T) {
	tracks := []*music.Track{
		{Title: "First", Source: music.SourceYouTube, Duration: 3 * time.Minute, Autoplay: true},
		{Title: "Second", Source: music.SourceYouTube, Duration: 4 * time.Minute, Autoplay: false},
	}

	outOn := FormatTrackList(tracks, player.RepeatOff, true, false)
	if !strings.Contains(outOn, "**Autoplay:** `On`") {
		t.Errorf("expected header to contain '**Autoplay:** `On`', got:\n%s", outOn)
	}
	if !strings.Contains(outOn, "[YouTube] 📻") {
		t.Errorf("expected autoplayed track to have '📻' badge, got:\n%s", outOn)
	}

	outOff := FormatTrackList(tracks, player.RepeatAll, false, true)
	if !strings.Contains(outOff, "**Autoplay:** `Off`") {
		t.Errorf("expected header to contain '**Autoplay:** `Off`', got:\n%s", outOff)
	}
}
