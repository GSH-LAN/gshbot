package emojihandler

import (
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
)

func pickRandomCustomEmoji(s *discordgo.Session, msg *discordgo.MessageCreate, foundEmoji *discordgo.Emoji) *discordgo.Emoji {
	var pick *discordgo.Emoji
	//log.Println("called picRandomCustomEmoji function with ", foundEmoji.Name)
	guildEmojis, err := s.GuildEmojis(msg.GuildID)

	if err == nil {
		for index, element := range guildEmojis {
			if element.ID == foundEmoji.ID {
				guildEmojis = append(guildEmojis[:index], guildEmojis[index+1:]...)
				break
			}
		}
		rand.New(rand.NewSource(time.Now().UnixNano()))
		randomIndex := rand.Intn(len(guildEmojis))
		pick = guildEmojis[randomIndex]
		//log.Println("picked random emoji ", pick.APIName())
	}
	return pick
}

func (h Handler) ReplyCommands(s *discordgo.Session, msg *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if msg.Author.ID == s.State.User.ID {
		return
	}

	emojis := msg.GetCustomEmojis()
	if len(emojis) >= 1 {
		//log.println("found cust emoji in msg")
		for _, element := range emojis {
			//log.Println(element.APIName())
			if element.Name == "moritz" {
				continue
			} else {
				random := pickRandomCustomEmoji(s, msg, element)
				//log.Println("got random emoji ", random.APIName())
				s.MessageReactionAdd(msg.ChannelID, msg.Message.ID, random.APIName())
			}
		}
	}
}
