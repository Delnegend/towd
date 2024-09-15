package kanban_handler

import (
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func Init(as *utils.AppState) {
	// works similar to how we create a new slash command using
	// appCmdInfo and appCmdHandler in AppState.
	localCmdInfo := make(
		[]*discordgo.ApplicationCommandOption, 0,
	)
	localCmdHandler := make(
		map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error,
	)

	list(as, &localCmdInfo, localCmdHandler)
	createItem(as, &localCmdInfo, localCmdHandler)
	createGroup(as, &localCmdInfo, localCmdHandler)
	moveItem(as, &localCmdInfo, localCmdHandler)
	deleteItem(as, &localCmdInfo, localCmdHandler)

	id := "kanban"
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Kanban management commands.",
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
