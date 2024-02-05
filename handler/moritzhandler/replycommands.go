package moritzhandler

import (
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (h Handler) ReplyCommands(s *discordgo.Session, msg *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if msg.Author.ID == s.State.User.ID {
		return
	}

	lowerContent := strings.ToLower(msg.Content)
	match, _ := regexp.MatchString("(m+o+r+i+t+z+)", lowerContent)
	if match {
		moritzEmoji := "moritz:269755553624883201"
		s.MessageReactionAdd(msg.ChannelID, msg.Message.ID, moritzEmoji)
		if msg.Author.ID == h.config.MoritzUserId {
			mo := msg.Author.ID
			message := "praise the lord <@" + mo + "> is here! :hearts:"
			s.ChannelMessageSend(msg.ChannelID, message)
		}
	}
}
