package utils

import (
	"log/slog"
	"os"
	"time"
)

type Config struct {
	port string

	jwtSecret string
	jwtExpire time.Duration

	discordGuildID      string
	discordBotToken     string
	discordClientId     string
	discordClientSecret string

	location *time.Location
}

func NewConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	secret := os.Getenv("SECRET")
	if secret == "" {
		slog.Warn("SECRET is not set")
		secret = "secret"
	}

	discordGuildID := os.Getenv("DISCORD_GUILD_ID")
	if discordGuildID == "" {
		slog.Error("DISCORD_GUILD_ID is not set")
		os.Exit(1)
	}

	discordBotToken := os.Getenv("DISCORD_APP_TOKEN")
	if discordBotToken == "" {
		slog.Error("DISCORD_APP_TOKEN is not set")
		os.Exit(1)
	}

	discordClientId := os.Getenv("DISCORD_CLIENT_ID")
	if discordClientId == "" {
		slog.Error("DISCORD_CLIENT_ID is not set")
		os.Exit(1)
	}

	discordClientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if discordClientSecret == "" {
		slog.Error("DISCORD_CLIENT_SECRET is not set")
		os.Exit(1)
	}

	discordUseRelativeTime := os.Getenv("DISCORD_USE_RELATIVE_TIME")
	if discordUseRelativeTime == "" {
		discordUseRelativeTime = "false"
	}

	timezoneStr := os.Getenv("TIMEZONE")
	var loc *time.Location
	var err error
	if timezoneStr != "" {
		loc, err = time.LoadLocation(timezoneStr)
		if err != nil {
			slog.Error("invalid timezone", "timezone", timezoneStr, "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("TIMEZONE is not set, using local timezone", "timezone", time.Local)
	}

	return &Config{
		port:   port,
		secret: secret,

		discordGuildID:      discordGuildID,
		discordBotToken:     discordBotToken,
		discordClientId:     discordClientId,
		discordClientSecret: discordClientSecret,

		location: loc,
	}
}

// Get PORT env, default to 8080
func (c *Config) GetPort() string {
	return c.port
}

// Get SECRET env, default to "secret"
func (c *Config) GetSecret() string {
	return c.secret
}

// Get DISCORD_GUILD_ID env
func (c *Config) GetDiscordGuildID() string {
	return c.discordGuildID
}

// Get DISCORD_APP_TOKEN env
func (c *Config) GetDiscordAppToken() string {
	return c.discordBotToken
}

// Get DISCORD_CLIENT_ID env
func (c *Config) GetDiscordClientId() string {
	return c.discordClientId
}

// Get DISCORD_CLIENT_SECRET env
func (c *Config) GetDiscordClientSecret() string {
	return c.discordClientSecret
}

// Get TIMEZONE env
func (c *Config) GetLocation() (*time.Location, error) {
	if c.location == nil {
		return time.Local, nil
	}
	return c.location, nil
}
