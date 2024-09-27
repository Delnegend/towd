package utils

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	port string

	discordGuildID      string
	discordAppToken     string
	discordClientId     string
	discordClientSecret string

	location   *time.Location
	groqApiKey string

	hostname           string
	staticWebClientDir string

	eventNotifyInterval      time.Duration
	calendarUpdateInterval   time.Duration
	metricCollectionInterval time.Duration

	dev bool
}

func NewConfig() *Config {
	return &Config{
		port: func() string {
			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			slog.Debug("env", "PORT", port)
			return port
		}(),

		discordGuildID: func() string {
			discordGuildID := os.Getenv("DISCORD_GUILD_ID")
			if discordGuildID == "" {
				slog.Error("DISCORD_GUILD_ID is not set")
				os.Exit(1)
			}
			slog.Debug("env", "DISCORD_GUILD_ID", discordGuildID)
			return discordGuildID
		}(),
		discordAppToken: func() string {
			discordAppToken := os.Getenv("DISCORD_APP_TOKEN")
			if discordAppToken == "" {
				slog.Error("DISCORD_APP_TOKEN is not set")
				os.Exit(1)
			}
			slog.Debug("env", "DISCORD_APP_TOKEN", discordAppToken[0:3]+"...")
			return discordAppToken
		}(),
		discordClientId: func() string {
			discordClientId := os.Getenv("DISCORD_CLIENT_ID")
			if discordClientId == "" {
				slog.Error("DISCORD_CLIENT_ID is not set")
				os.Exit(1)
			}
			slog.Debug("env", "DISCORD_CLIENT_ID", discordClientId)
			return discordClientId
		}(),
		discordClientSecret: func() string {
			discordClientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
			if discordClientSecret == "" {
				slog.Error("DISCORD_CLIENT_SECRET is not set")
				os.Exit(1)
			}
			return discordClientSecret
		}(),

		location: func() *time.Location {
			timezoneStr := os.Getenv("TIMEZONE")
			var loc *time.Location
			var err error
			switch timezoneStr {
			case "":
				slog.Warn("TIMEZONE is not set, using local timezone", "timezone", time.Local)
				loc = time.Local
			case "UTC":
				slog.Warn("TIMEZONE is set to UTC, using UTC timezone", "timezone", time.UTC)
				loc = time.UTC
			default:
				loc, err = time.LoadLocation(timezoneStr)
				if err != nil {
					slog.Error("invalid timezone", "timezone", timezoneStr, "error", err)
					os.Exit(1)
				}
			}
			slog.Debug("env", "TIMEZONE", timezoneStr)
			return loc
		}(),
		groqApiKey: func() string {
			groqApiKey := os.Getenv("GROQ_API_KEY")
			if groqApiKey == "" {
				slog.Error("GROQ_API_KEY is not set")
				os.Exit(1)
			}
			slog.Debug("env", "GROQ_API_KEY", groqApiKey[0:3]+"...")
			return groqApiKey
		}(),

		staticWebClientDir: func() string {
			staticWebClientDir := os.Getenv("STATIC_WEB_CLIENT_DIR")
			if staticWebClientDir == "" {
				slog.Error("STATIC_WEB_CLIENT_DIR is not set")
				os.Exit(1)
			}
			info, err := os.Stat(staticWebClientDir)
			if err != nil {
				slog.Error("can't get info of STATIC_WEB_CLIENT_DIR", "error", err)
				os.Exit(1)
			}
			if !info.IsDir() {
				slog.Error("STATIC_WEB_CLIENT_DIR is not a directory", "error", err)
				os.Exit(1)
			}

			slog.Debug("env", "STATIC_WEB_CLIENT_DIR", staticWebClientDir)
			return filepath.Clean(staticWebClientDir)
		}(),
		hostname: func() string {
			hostname := os.Getenv("HOSTNAME")
			if hostname == "" {
				slog.Error("HOSTNAME is not set")
				os.Exit(1)
			}
			slog.Debug("env", "HOSTNAME", hostname)
			return hostname
		}(),

		eventNotifyInterval: func() time.Duration {
			eventNotifyInterval := os.Getenv("EVENT_NOTIFY_INTERVAL")
			if eventNotifyInterval == "" {
				slog.Warn("EVENT_NOTIFY_INTERVAL is not set, using default value", "interval", time.Minute)
				return time.Minute
			}
			duration, err := time.ParseDuration(eventNotifyInterval)
			if err != nil {
				slog.Error("invalid EVENT_NOTIFY_INTERVAL", "error", err)
				os.Exit(1)
			}
			slog.Debug("env", "EVENT_NOTIFY_INTERVAL", eventNotifyInterval, "duration", duration)
			return duration
		}(),
		calendarUpdateInterval: func() time.Duration {
			calendarUpdateInterval := os.Getenv("CALENDAR_UPDATE_INTERVAL")
			if calendarUpdateInterval == "" {
				slog.Warn("CALENDAR_UPDATE_INTERVAL is not set, using default value", "interval", time.Hour)
				return time.Hour
			}
			duration, err := time.ParseDuration(calendarUpdateInterval)
			if err != nil {
				slog.Error("invalid CALENDAR_UPDATE_INTERVAL", "error", err)
				os.Exit(1)
			}
			slog.Debug("env", "CALENDAR_UPDATE_INTERVAL", calendarUpdateInterval, "duration", duration)
			return duration
		}(),
		metricCollectionInterval: func() time.Duration {
			metricCollectionInterval := os.Getenv("METRIC_COLLECTION_INTERVAL")
			if metricCollectionInterval == "" {
				slog.Warn("METRIC_COLLECTION_INTERVAL is not set, using default value", "interval", time.Second*5)
				return time.Second * 2
			}
			duration, err := time.ParseDuration(metricCollectionInterval)
			if err != nil {
				slog.Error("invalid METRIC_COLLECTION_INTERVAL", "error", err)
				os.Exit(1)
			}
			slog.Debug("env", "METRIC_COLLECTION_INTERVAL", metricCollectionInterval, "duration", duration)
			return duration
		}(),

		dev: func() bool {
			dev := os.Getenv("DEV")
			if dev == "" {
				slog.Warn("DEV is not set, using default value", "dev", false)
				return false
			}
			slog.Debug("env", "DEV", dev)
			return dev == "true"
		}(),
	}
}

// Get PORT env, default to 8080
func (c *Config) GetPort() string {
	return c.port
}

// Get DISCORD_GUILD_ID env
func (c *Config) GetDiscordGuildID() string {
	return c.discordGuildID
}

// Get DISCORD_APP_TOKEN env
func (c *Config) GetDiscordAppToken() string {
	return c.discordAppToken
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
func (c *Config) GetLocation() *time.Location {
	return c.location
}

// Get GROQ_API_KEY env
func (c *Config) GetGroqApiKey() string {
	return c.groqApiKey
}

// Get STATIC_WEB_CLIENT_DIR env
func (c *Config) GetStaticWebClientDir() string {
	return c.staticWebClientDir
}

// Get HOSTNAME env
func (c *Config) GetHostname() string {
	return c.hostname
}

// Get EVENT_NOTIFY_INTERVAL env
func (c *Config) GetEventNotifyInterval() time.Duration {
	return c.eventNotifyInterval
}

// Get CALENDAR_UPDATE_INTERVAL env
func (c *Config) GetCalendarUpdateInterval() time.Duration {
	return c.calendarUpdateInterval
}

// Get METRIC_COLLECTION_INTERVAL env
func (c *Config) GetMetricCollectionInterval() time.Duration {
	return c.metricCollectionInterval
}

// Get DEV env
func (c *Config) GetDev() bool {
	return c.dev
}
