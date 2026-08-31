package moritzhandler_test

import (
	"testing"

	"gshlan/gshbot/config"
	"gshlan/gshbot/handler/moritzhandler"

	"github.com/bwmarrin/discordgo"
)

// newTestSession builds a discordgo.Session without opening a real gateway or
// REST connection, so ReplyCommands can be exercised for branches that never
// reach out to the Discord API.
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
	h := moritzhandler.New(&config.Discord{MoritzUserId: "someone"})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-1",
		ChannelID: "chan-1",
		Content:   "moritz moritz moritz",
		Author:    &discordgo.User{ID: "bot-id"},
	}}

	// Must return before touching the Discord API - a panic or hang would fail the test.
	h.ReplyCommands(s, msg)
}

func TestReplyCommands_NoMatch(t *testing.T) {
	s := newTestSession(t, "bot-id")
	h := moritzhandler.New(&config.Discord{MoritzUserId: "someone"})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-2",
		ChannelID: "chan-1",
		Content:   "hello world",
		Author:    &discordgo.User{ID: "other-user"},
	}}

	h.ReplyCommands(s, msg)
}
