package emojihandler

import (
	"gshlan/gshbot/config"
)

type Handler struct {
	config  *config.Discord
	actions []func()
}

func New(cfg *config.Discord) *Handler {
	return &Handler{
		config: cfg,
	}
}
