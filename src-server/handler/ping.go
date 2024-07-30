package handler

import (
	"fmt"
	"log/slog"
	"runtime"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func Ping(as *utils.AppState) {
	id := "ping"
	as.AddAppCmdHandler(id, pingHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "A ping command.",
	})
}

func pingHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memUsage := float64(m.Sys) / 1024 / 1024

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "Pong!",
						Footer: &discordgo.MessageEmbedFooter{
							Text: i.GuildID,
						},
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Uptime",
								Value: as.GetUptime().String(),
							},
							{
								Name:   "Latency",
								Value:  fmt.Sprintf("%dms", s.HeartbeatLatency().Milliseconds()),
								Inline: true,
							},
							{
								Name:   "Go version",
								Value:  runtime.Version(),
								Inline: true,
							},
							{
								Name:   "Memory",
								Value:  fmt.Sprintf("%.2fMB", memUsage),
								Inline: true,
							},
						},
					},
				},
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "ping", "err", err)
		}
		return nil
	}
}
