package messagehandler

import (
	"gshlan/gshbot/extension/rss"
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

	if strings.HasPrefix(msg.Content, h.config.Prefix+"rss") && len(strings.Split(msg.Content, " ")) >= 3 {
		if !hasManageGuildPermission(s, msg) {
			s.ChannelMessageSend(msg.ChannelID, "you need the 'Manage Server' permission to manage RSS feeds")
			return
		}

		var badChannelId string
		var channelId string
		if strings.Split(msg.Content, " ")[1] == "add" {
			name := strings.Split(msg.Content, " ")[2]
			url := strings.Split(msg.Content, " ")[3]
			if strings.Contains(url, "http://") || strings.Contains(url, "https://") {
				s.ChannelMessageSend(msg.ChannelID, "try to add the following feed name: "+name+" url: "+url)
				if len(strings.Split(msg.Content, " ")) == 5 {
					badChannelId = strings.Split(msg.Content, " ")[4]
					r, _ := regexp.Compile("\\d{18}")
					channelId = r.FindString(badChannelId)
				} else {
					channelId = msg.ChannelID
				}
				added, returnErr := rss.AddUrlToList(name, url, channelId, h.config)
				if added && returnErr == nil {
					s.ChannelMessageSend(msg.ChannelID, "feed added successfully")
				} else {
					s.ChannelMessageSend(msg.ChannelID, "an error occurred: "+returnErr.Error())
				}
			} else {
				s.ChannelMessageSend(msg.ChannelID, "please use a proper URL!")
			}
		} else if strings.Split(msg.Content, " ")[1] == "del" {
			name := strings.Split(msg.Content, " ")[2]
			if len(strings.Split(msg.Content, " ")) == 4 {
				channelId = strings.Split(msg.Content, " ")[3]
			} else {
				channelId = msg.ChannelID
			}
			removed, removeErr := rss.RemoveFeedFromList(name, channelId, h.config)
			if removed && removeErr == nil {
				s.ChannelMessageSend(msg.ChannelID, "feed removed successfully")
			} else {
				s.ChannelMessageSend(msg.ChannelID, "an error occurred: "+removeErr.Error())
			}
		}

	}
}

// hasManageGuildPermission reports whether the message author has the
// Discord "Manage Server" permission in the channel the command was sent in.
func hasManageGuildPermission(s *discordgo.Session, msg *discordgo.MessageCreate) bool {
	perms, err := s.UserChannelPermissions(msg.Author.ID, msg.ChannelID)
	if err != nil {
		return false
	}
	return perms&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild
}
