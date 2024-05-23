package handler

import (
	"fmt"
	"runtime"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func Ping(appState *utils.AppState) {
	appState.AppCmdHandler["ping"] = pingHandler()
	appState.AppCmdInfo["ping"] = &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "A ping command.",
	}
}

func pingHandler() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		latency := fmt.Sprintf("%.2fms", float64(s.HeartbeatLatency().Nanoseconds())/1000000)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Content: "- Server ID: `" + i.GuildID + "`" +
					"\n- Latency: `" + latency + "`" +
					"\n- Go version: `" + runtime.Version()[2:] + "`",
			},
		})
	}
}
