package event_handler

import (
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

// Init injects one "event" slash command with multiple subcommands
// into appCmdInfo and appCmdHandler in AppState.
func Init(as *utils.AppState) {
	// works similar to how we create a new slash command using
	// appCmdInfo and appCmdHandler in AppState.
	localCmdInfo := make(
		[]*discordgo.ApplicationCommandOption, 0,
	)
	localCmdHandler := make(
		map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error,
	)

	// injecting info and handler into 2 local maps
	createManual(as, &localCmdInfo, localCmdHandler)
	create(as, &localCmdInfo, localCmdHandler)
	delete(as, &localCmdInfo, localCmdHandler)
	list(as, &localCmdInfo, localCmdHandler)
	modify(as, &localCmdInfo, localCmdHandler)

	as.AddAppCmdInfo("event", &discordgo.ApplicationCommand{
		Name:        "event",
		Description: "Event management commands.",
		Options:     localCmdInfo,
	})
	as.AddAppCmdHandler("event", func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		data := i.ApplicationCommandData()
		if handler, ok := localCmdHandler[data.Options[0].Name]; ok {
			return handler(s, i)
		}
		return nil
	})
}
