package auth_handler

import (
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

// Init injects one "auth" slash command with multiple subcommands
// into appCmdInfo and appCmdHandler in AppState.
func Init(as *utils.AppState) {
	localCmdInfo := make(
		[]*discordgo.ApplicationCommandOption, 0,
	)
	localCmdHandler := make(
		map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error,
	)

	login(as, &localCmdInfo, localCmdHandler)
	totpCreate(as, &localCmdInfo, localCmdHandler)
	totpCheck(as, &localCmdInfo, localCmdHandler)

	id := "auth"
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Authentication commands.",
		Options:     localCmdInfo,
	})
	as.AddAppCmdHandler(id, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		data := i.ApplicationCommandData()
		if handler, ok := localCmdHandler[data.Options[0].Name]; ok {
			return handler(s, i)
		}
		return nil
	})
}
