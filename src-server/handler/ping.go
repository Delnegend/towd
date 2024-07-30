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
	as.AddAppCmdHandler(id, pingHandler())
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "A ping command.",
	})
}

func pingHandler() func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {

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
								Name:   "Latency",
								Value:  fmt.Sprintf("%dms", s.HeartbeatLatency().Milliseconds()),
								Inline: true,
							},
							{
								Name:   "Go version",
								Value:  runtime.Version()[2:],
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
