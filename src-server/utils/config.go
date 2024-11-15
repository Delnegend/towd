package utils

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	port string

	discordGuildID      string
	discordAppToken     string
	discordClientId     string
	discordClientSecret string

	groqApiKey   string
	geminiApiKey string
	llmProvider  LLMProvider

	location           *time.Location
	staticWebClientDir string

	eventNotifyInterval      time.Duration
	calendarUpdateInterval   time.Duration
	metricCollectionInterval time.Duration
}

func NewConfig() *Config {
	return &Config{
		port: func() string {
			port := os.Getenv("SERVER_PORT")
			if port == "" {
				port = "8080"
			}
			slog.Debug("env", "SERVER_PORT", port)
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

		groqApiKey: func() string {
			groqApiKey := os.Getenv("GROQ_API_KEY")
			if groqApiKey == "" {
				return ""
			}
			slog.Debug("env", "GROQ_API_KEY", groqApiKey[0:3]+"...")
			return groqApiKey
		}(),
		geminiApiKey: func() string {
			geminiApiKey := os.Getenv("GEMINI_API_KEY")
			if geminiApiKey == "" {
				return ""
			}
			slog.Debug("env", "GEMINI_API_KEY", geminiApiKey[0:3]+"...")
			return geminiApiKey
		}(),
		llmProvider: func() LLMProvider {
			llmProvider := strings.ToLower(os.Getenv("LLM_PROVIDER"))
			if llmProvider == "" {
				slog.Warn("LLM_PROVIDER is not set, using default value", "provider", LLMProviderGroq)
				return LLMProviderGroq
			}
			switch llmProvider {
			case "groq":
				slog.Warn("LLM_PROVIDER is set to groq, using groq provider", "provider", LLMProviderGroq)
				return LLMProviderGroq
			case "gemini":
				slog.Warn("LLM_PROVIDER is set to gemini, using gemini provider", "provider", LLMProviderGemini)
				return LLMProviderGemini
			default:
				slog.Error("invalid LLM_PROVIDER", "provider", llmProvider)
				os.Exit(1)
			}
			return LLMProvider("")
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
		staticWebClientDir: func() string {
			staticWebClientDir := os.Getenv("STATIC_WEB_CLIENT_DIR")
			if staticWebClientDir == "" {
				slog.Error("STATIC_WEB_CLIENT_DIR is not set")
				os.Exit(1)
			}
			info, err := os.Stat(staticWebClientDir)
			if err != nil {
				slog.Error("can't get info of STATIC_WEB_CLIENT_DIR", "error", err)
				return ""
			}
			if !info.IsDir() {
				slog.Error("STATIC_WEB_CLIENT_DIR is not a directory", "error", err)
				return ""
			}

			slog.Debug("env", "STATIC_WEB_CLIENT_DIR", staticWebClientDir)
			return filepath.Clean(staticWebClientDir)
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

// Get GROQ_API_KEY env
func (c *Config) GetGroqApiKey() string {
	return c.groqApiKey
}

// Get GEMINI_API_KEY env
func (c *Config) GetGeminiApiKey() string {
	return c.geminiApiKey
}

// Get LLM_PROVIDER env
func (c *Config) GetLLMProvider() LLMProvider {
	return c.llmProvider
}

// Get TIMEZONE env
func (c *Config) GetLocation() *time.Location {
	return c.location
}

// Get STATIC_WEB_CLIENT_DIR env
func (c *Config) GetStaticWebClientDir() string {
	return c.staticWebClientDir
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
