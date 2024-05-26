package handler

import (
	"fmt"
	"net/url"
	"towd/src-server/ical"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func ImportCalendar(appState *utils.AppState) {
	appState.AppCmdHandler["import-calendar"] = importCalendarHandler(appState)
	appState.AppCmdInfo["import-calendar"] = &discordgo.ApplicationCommand{
		Name:        "import-calendar",
		Description: "Import an external calendar.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "The URL of the external calendar",
				Required:    true,
			},
		},
	}
}

func importCalendarHandler(appState *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		url_ := i.ApplicationCommandData().Options[0].StringValue()
		if _, err := url.ParseRequestURI(url_); err != nil {
			utils.InteractRespHiddenReply(s, i, "Please provide a valid URL.")
			return
		}

		_, err := ical.UnmarshalUrl(url_)
		if err != nil {
			utils.InteractRespHiddenReply(s, i, fmt.Sprintf("Failed to fetch/unmarshal the calendar. %s", err))
			return
		}
	}
}
