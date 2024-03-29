package websocketserver

import (
	"gshlan/gshbot/config"

	"github.com/bwmarrin/discordgo"

	"gshlan/gshbot/extension/rss"
	"gshlan/gshbot/extension/status"
)

type Handler interface {
	Register(session *discordgo.Session)
	DeRegister(session *discordgo.Session)
}

type Server struct {
	Session  *discordgo.Session
	config   *config.Config
	handlers []Handler
}

func New(cfg *config.Config, handlers []Handler, intent discordgo.Intent) *Server {
	s, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {

		panic(err)
	}

	s.Identify.Intents = intent

	return &Server{
		config:   cfg,
		Session:  s,
		handlers: handlers,
	}
}

func (s Server) Serve() {
	err := s.Session.Open()
	if err != nil {
		panic(err)
	}

	for _, h := range s.handlers {
		h.Register(s.Session)
	}

	rss.LoadFeeds(&s.config.Discord)
	rss.ConfigureRSSFeeds(s.Session)
	status.SetBotStatus(s.Session)
}

func (s Server) Shutdown() {
	for _, h := range s.handlers {
		h.DeRegister(s.Session)
	}

	s.Session.Close()
}
