package status

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func SetBotStatus(s *discordgo.Session) {
	err := s.UpdateStatusComplex(discordgo.UpdateStatusData{
		IdleSince: nil,
		Activities: []*discordgo.Activity{{
			Name:  "gshlan",
			State: "www.gsh-lan.com",
			Type:  discordgo.ActivityTypeCustom,
		}},
		AFK:    false,
		Status: "online",
	})

	if err != nil {
		log.Panic("got error ", err)
	}
}
