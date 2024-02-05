package config

type Discord struct {
	Name         string `koanf:"name"`
	GuildID      string `koanf:"guild_id"`
	Token        string `koanf:"token"`
	Prefix       string `koanf:"prefix"`
	DBName       string `koanf:"dbname"`
	DBColName    string `koanf:"dbcolname"`
	MoritzUserId string `koanf:"moritz_user_id"`
}

type Config struct {
	Discord Discord `koanf:"discord"`
}

func New(discord Discord) Config {
	return Config{
		Discord: discord,
	}
}
