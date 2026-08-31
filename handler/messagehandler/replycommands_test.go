package messagehandler_test

import (
	"testing"

	"gshlan/gshbot/config"
	"gshlan/gshbot/handler/messagehandler"

	"github.com/bwmarrin/discordgo"
)

func newTestSession(t *testing.T, botID string) *discordgo.Session {
	t.Helper()
	s, err := discordgo.New("Bot faketoken")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	s.State.User = &discordgo.User{ID: botID}
	return s
}

func TestReplyCommands_IgnoresOwnMessages(t *testing.T) {
	s := newTestSession(t, "bot-id")
	h := messagehandler.New(&config.Discord{Prefix: "!"})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-1",
		ChannelID: "chan-1",
		Content:   "!rss add news https://example.com/rss",
		Author:    &discordgo.User{ID: "bot-id"},
	}}

	h.ReplyCommands(s, msg)
}

func TestReplyCommands_IgnoresMessagesWithoutRssPrefix(t *testing.T) {
	s := newTestSession(t, "bot-id")
	h := messagehandler.New(&config.Discord{Prefix: "!"})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-2",
		ChannelID: "chan-1",
		Content:   "hello world",
		Author:    &discordgo.User{ID: "other-user"},
	}}

	h.ReplyCommands(s, msg)
}

func TestReplyCommands_IgnoresIncompleteRssCommand(t *testing.T) {
	s := newTestSession(t, "bot-id")
	h := messagehandler.New(&config.Discord{Prefix: "!"})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-3",
		ChannelID: "chan-1",
		Content:   "!rss add",
		Author:    &discordgo.User{ID: "other-user"},
	}}

	// Fewer than 3 space-separated parts - the handler must not attempt any API call.
	h.ReplyCommands(s, msg)
}
