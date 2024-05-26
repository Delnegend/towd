package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

func CreateEvent(appState *utils.AppState) {
	appState.AppCmdHandler["create-event"] = createEventHandler(appState)
	appState.AppCmdInfo["create-event"] = &discordgo.ApplicationCommand{
		Name:        "create-event",
		Description: "Create a calendar event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "title",
				Description: "The title of the event",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start",
				Description: "Starting time of the event",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end",
				Description: "Ending time of the event",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "details",
				Description: "Detailed description of the event",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "location",
				Description: "Location of the event",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "invitees",
				Description: "Users to invite to the event",
				Required:    false,
			},
		},
	}
}

func createEventHandler(appState *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// prepare the options
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		// new model, init vars
		eventModel := model.Event{
			ID: uuid.New(),
		}
		var startDate, endDate string
		var invitees *discordgo.User

		// get vars from options
		if opt, ok := optionMap["title"]; ok {
			eventModel.Title = opt.StringValue()
		}
		if opt, ok := optionMap["details"]; ok {
			eventModel.Details = opt.StringValue()
		}
		if opt, ok := optionMap["start"]; ok {
			startDate = opt.StringValue()
		}
		if opt, ok := optionMap["end"]; ok {
			endDate = opt.StringValue()
		}
		if opt, ok := optionMap["location"]; ok {
			eventModel.Location = opt.StringValue()
		}
		if opt, ok := optionMap["invitees"]; ok {
			invitees = opt.UserValue(s)
		}

		// parse date
		parsedStartDate, err := appState.When.Parse(startDate, time.Now())
		if err != nil {
			utils.InteractRespHiddenReply(s, i, "Can't parse start date")
			return
		}
		parsedEndDate, err := appState.When.Parse(endDate, time.Now())
		if err != nil {
			utils.InteractRespHiddenReply(s, i, "Can't parse end date")
			return
		}
		eventModel.StartDate = parsedStartDate.Time
		eventModel.EndDate = parsedEndDate.Time

		// validate date
		if eventModel.StartDate.After(eventModel.EndDate) {
			utils.InteractRespHiddenReply(s, i, "Start date is after end date")
			return
		}

		slog.Debug("received event creation request", "event", eventModel)

		appState.EventQueue[eventModel.ID] = utils.MsgComponentInfo{
			DateAdded: time.Now(),
			Data:      eventModel,
		}

		// custom IDs
		yesCustomId := "yes-" + eventModel.ID.String()
		cancelCustomId := "cancel-" + eventModel.ID.String()

		// ask to confirm
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: func() string {
					content := fmt.Sprintf("- Title: *%s*", eventModel.Title)
					if eventModel.Details != "" {
						content += fmt.Sprintf("\n- Details: *%s*", eventModel.Details)
					}
					content += fmt.Sprintf("\n- Start: <t:%d:F>", parsedStartDate.Time.Unix())
					content += fmt.Sprintf("\n- End: <t:%d:F>", parsedEndDate.Time.Unix())
					if eventModel.Location != "" {
						content += fmt.Sprintf("\n- Location: *%s*", eventModel.Location)
					}
					if invitees != nil {
						content += fmt.Sprintf("\n- Invitees: *%s*", invitees.Username)
					}
					content += "\n\nIs this correct?"
					return content
				}(),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Yes",
								Style:    discordgo.SuccessButton,
								CustomID: yesCustomId,
							},
							discordgo.Button{
								Label:    "Cancel",
								Style:    discordgo.PrimaryButton,
								CustomID: cancelCustomId,
							},
						},
					},
				},
			},
		})

		// yes handler
		appState.AppCmdHandler[yesCustomId] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			item, ok := appState.EventQueue[uuid.MustParse(i.MessageComponentData().CustomID[4:])]
			if !ok {
				utils.InteractRespHiddenReply(s, i, "Event expired")
				return
			}
			event := item.Data.(model.Event)

			if _, err := appState.BunDb.NewInsert().Model(&event).Exec(context.Background()); err != nil {
				slog.Error("cannot insert event", "error", err)
				utils.InteractRespHiddenReply(s, i, fmt.Sprintf("Cannot create event: `%s`", err.Error()))
				return
			}
			utils.InteractRespHiddenReply(s, i, "Event created!")
			delete(appState.AppCmdHandler, yesCustomId)
			delete(appState.AppCmdHandler, cancelCustomId)
		}

		// no handler
		appState.AppCmdHandler[cancelCustomId] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			delete(appState.EventQueue, uuid.MustParse(i.MessageComponentData().CustomID[7:]))
			utils.InteractRespHiddenReply(s, i, "Cancelled event creation")
			delete(appState.AppCmdHandler, yesCustomId)
			delete(appState.AppCmdHandler, cancelCustomId)
		}
	}
}
