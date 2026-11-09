package messagehandler

import (
	"gshlan/gshbot/extension/rss"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func parseChannelID(raw, defaultID string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "<#")
	raw = strings.TrimSuffix(raw, ">")
	if raw == "" {
		return defaultID
	}
	return raw
}

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

		parts := strings.Split(msg.Content, " ")
		subCmd := parts[1]

		var channelId string
		switch subCmd {
		case "add":
			name := parts[2]
			url := parts[3]
			if strings.Contains(url, "http://") || strings.Contains(url, "https://") {
				s.ChannelMessageSend(msg.ChannelID, "try to add the following feed name: "+name+" url: "+url)
				if len(parts) >= 5 {
					channelId = parseChannelID(parts[4], msg.ChannelID)
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
		case "del":
			name := parts[2]
			if len(parts) >= 4 {
				channelId = parseChannelID(parts[3], msg.ChannelID)
			} else {
				channelId = msg.ChannelID
			}
			removed, removeErr := rss.RemoveFeedFromList(name, channelId, h.config)
			if removed && removeErr == nil {
				s.ChannelMessageSend(msg.ChannelID, "feed removed successfully")
			} else {
				s.ChannelMessageSend(msg.ChannelID, "an error occurred: "+removeErr.Error())
			}
		case "pause", "disable":
			name := parts[2]
			if len(parts) >= 4 {
				channelId = parseChannelID(parts[3], msg.ChannelID)
			} else {
				channelId = msg.ChannelID
			}
			paused, pauseErr := rss.PauseFeed(s, name, channelId, h.config)
			if paused && pauseErr == nil {
				s.ChannelMessageSend(msg.ChannelID, "feed paused successfully")
			} else {
				s.ChannelMessageSend(msg.ChannelID, "an error occurred: "+pauseErr.Error())
			}
		case "resume", "enable":
			name := parts[2]
			if len(parts) >= 4 {
				channelId = parseChannelID(parts[3], msg.ChannelID)
			} else {
				channelId = msg.ChannelID
			}
			resumed, resumeErr := rss.ResumeFeed(s, name, channelId, h.config)
			if resumed && resumeErr == nil {
				s.ChannelMessageSend(msg.ChannelID, "feed resumed successfully")
			} else {
				s.ChannelMessageSend(msg.ChannelID, "an error occurred: "+resumeErr.Error())
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
