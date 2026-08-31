package emojihandler_test

import (
	"testing"

	"gshlan/gshbot/config"
	"gshlan/gshbot/handler/emojihandler"

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
	h := emojihandler.New(&config.Discord{})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-1",
		ChannelID: "chan-1",
		Content:   "<:someemoji:123456789012345678>",
		Author:    &discordgo.User{ID: "bot-id"},
	}}

	h.ReplyCommands(s, msg)
}

func TestReplyCommands_NoCustomEmoji(t *testing.T) {
	s := newTestSession(t, "bot-id")
	h := emojihandler.New(&config.Discord{})

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-2",
		ChannelID: "chan-1",
		Content:   "just a plain message without any emoji",
		Author:    &discordgo.User{ID: "other-user"},
	}}

	// No custom emoji in the content - must not call GuildEmojis/MessageReactionAdd.
	h.ReplyCommands(s, msg)
}
