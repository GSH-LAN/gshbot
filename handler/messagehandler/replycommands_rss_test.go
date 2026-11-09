package messagehandler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gshlan/gshbot/config"
	"gshlan/gshbot/extension/rss"

	"github.com/bwmarrin/discordgo"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockSession(t *testing.T, guildID, channelID, userID string, authorized bool) (*discordgo.Session, *[]string) {
	t.Helper()
	var roles []*discordgo.Role
	var memberRoles []string

	if authorized {
		roles = []*discordgo.Role{
			{ID: guildID, Permissions: 0},
			{ID: "role-manager", Permissions: discordgo.PermissionManageGuild},
		}
		memberRoles = []string{"role-manager"}
	} else {
		roles = []*discordgo.Role{
			{ID: guildID, Permissions: 0},
		}
	}

	s := newSessionWithGuild(t, guildID, channelID, userID, memberRoles, roles)
	if ch, err := s.State.Channel(channelID); err == nil && ch != nil {
		ch.Name = "news-channel"
	}

	var sentMessages []string
	s.Client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var payload struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(body, &payload)
		sentMessages = append(sentMessages, payload.Content)

		respJSON := `{"id": "123456789012345678", "channel_id": "` + channelID + `", "content": "` + payload.Content + `"}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	return s, &sentMessages
}

func TestReplyCommands_PauseAndResume_Unauthorized(t *testing.T) {
	const guildID, channelID, userID = "guild-1", "123456789012345678", "user-1"
	s, sentMessages := newMockSession(t, guildID, channelID, userID, false)
	cfg := &config.Discord{Prefix: "!", DBName: "testdb", DBColName: "feeds"}
	h := New(cfg)

	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss pause news",
			Author:    &discordgo.User{ID: userID},
		},
	}

	h.ReplyCommands(s, msg)

	if len(*sentMessages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(*sentMessages))
	}
	if !strings.Contains((*sentMessages)[0], "Manage Server") {
		t.Fatalf("expected permission warning, got: %s", (*sentMessages)[0])
	}
}

func TestReplyCommands_PauseAndResume_Authorized(t *testing.T) {
	const guildID, channelID, userID = "guild-1", "123456789012345678", "user-1"
	t.Chdir(t.TempDir())

	s, sentMessages := newMockSession(t, guildID, channelID, userID, true)
	cfg := &config.Discord{Prefix: "!", DBName: "testdb", DBColName: "feeds"}
	h := New(cfg)

	// Seed feed
	if _, err := rss.AddUrlToList("news", "https://example.com/rss", channelID, cfg); err != nil {
		t.Fatalf("failed to seed feed: %v", err)
	}

	// 1. Pause feed
	pauseMsg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss pause news",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, pauseMsg)

	if len(*sentMessages) != 1 || !strings.Contains((*sentMessages)[0], "feed paused successfully") {
		t.Fatalf("expected pause success message, got: %v", *sentMessages)
	}

	// 2. Resume feed
	resumeMsg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss resume news",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, resumeMsg)

	if len(*sentMessages) != 2 || !strings.Contains((*sentMessages)[1], "feed resumed successfully") {
		t.Fatalf("expected resume success message, got: %v", *sentMessages)
	}

	// 3. Disable alias
	disableMsg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss disable news",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, disableMsg)

	if len(*sentMessages) != 3 || !strings.Contains((*sentMessages)[2], "feed paused successfully") {
		t.Fatalf("expected disable success message, got: %v", *sentMessages)
	}

	// 4. Enable alias
	enableMsg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss enable news",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, enableMsg)

	if len(*sentMessages) != 4 || !strings.Contains((*sentMessages)[3], "feed resumed successfully") {
		t.Fatalf("expected enable success message, got: %v", *sentMessages)
	}
}

func TestReplyCommands_List_Empty(t *testing.T) {
	const guildID, channelID, userID = "guild-1", "123456789012345678", "user-1"
	t.Chdir(t.TempDir())

	s, sentMessages := newMockSession(t, guildID, channelID, userID, true)
	cfg := &config.Discord{Prefix: "!", DBName: "testdb", DBColName: "feeds"}
	h := New(cfg)

	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss list",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, msg)

	if len(*sentMessages) != 1 || !strings.Contains((*sentMessages)[0], "no RSS feeds configured") {
		t.Fatalf("expected 'no RSS feeds configured' message, got: %v", *sentMessages)
	}
}

func TestReplyCommands_List_WithFeeds(t *testing.T) {
	const guildID, channelID, userID = "guild-1", "123456789012345678", "user-1"
	t.Chdir(t.TempDir())

	s, sentMessages := newMockSession(t, guildID, channelID, userID, true)
	cfg := &config.Discord{Prefix: "!", DBName: "testdb", DBColName: "feeds"}
	h := New(cfg)

	if _, err := rss.AddUrlToList("news", "https://example.com/rss", channelID, cfg); err != nil {
		t.Fatalf("failed to seed feed: %v", err)
	}

	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: channelID,
			Content:   "!rss list",
			Author:    &discordgo.User{ID: userID},
		},
	}
	h.ReplyCommands(s, msg)

	if len(*sentMessages) != 1 {
		t.Fatalf("expected 1 message, got: %v", *sentMessages)
	}
	out := (*sentMessages)[0]
	if !strings.Contains(out, "Configured RSS Feeds (1):") ||
		!strings.Contains(out, "news") ||
		!strings.Contains(out, "https://example.com/rss") ||
		!strings.Contains(out, "#news-channel") ||
		!strings.Contains(out, channelID) ||
		!strings.Contains(out, "active") {
		t.Fatalf("unexpected list output: %s", out)
	}
}
