package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func CreateEvent(as *utils.AppState) {
	id := "create-event"
	as.AddAppCmdHandler(id, createEventHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
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
				Description: "When the event starts",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end",
				Description: "When the event ends",
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
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "invitees",
				Description: "Invitees of the event, each separated by comma",
				Required:    false,
			},
		},
	})
}

func createEventHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		// #region | send a model
		eventID := uuid.New().String()
		customID := "create-event-" + eventID

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: customID,
				Title:    "Create Event",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "title",
								Label:    "Title",
								Style:    discordgo.TextInputShort,
								Required: true,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "description",
								Label:    "Description",
								Style:    discordgo.TextInputParagraph,
								Required: false,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "start",
								Label:    "Start date",
								Style:    discordgo.TextInputShort,
								Required: true,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "end",
								Label:    "End date",
								Style:    discordgo.TextInputShort,
								Required: true,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "event-location",
								Label:    "Location",
								Style:    discordgo.TextInputShort,
								Required: false,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID: "url",
								Label:    "URL",
								Style:    discordgo.TextInputShort,
								Required: false,
							},
						},
					},
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "invitees",
								Label:       "Invitees",
								Style:       discordgo.TextInputShort,
								Required:    false,
								Placeholder: "separate by comma",
							},
						},
					},
				},
			},
		})
		dataMap := make(map[string]string, 0)
		var wg sync.WaitGroup
		wg.Add(1)
		as.AddAppCmdHandler(customID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			defer wg.Done()
			for _, opt := range i.ModalSubmitData().Components {
				actionRow, ok := opt.(*discordgo.ActionsRow)
				if !ok {
					continue
				}
				if len(actionRow.Components) != 1 {
					continue
				}
				textInput, ok := actionRow.Components[0].(*discordgo.TextInput)
				if !ok {
					continue
				}
				value := textInput.Value
				dataMap[textInput.CustomID] = value
			}
			return nil
		})
		defer as.RemoveAppCmdHandler(customID)
		wg.Wait()
		// #endregion

		// #region | parse and collect data
		eModel := new(model.MasterEvent)
		eModel.ID = eventID
		eModel.Organizer = i.Member.User.Username
		invitees := make([]string, 0)
		for k, v := range dataMap {
			switch k {
			case "title":
				eModel.Summary = v
			case "description":
				eModel.Description = v
			case "start":
				if result, err := as.When.Parse(v, time.Now()); err != nil {
					msg := fmt.Sprintf("Can't parse start date\n```\n%s\n```", err.Error())
					if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content: &msg,
					}); err != nil {
						slog.Error("can't respond", "handler", "create-event", "content", "parse-date-error", "error", err)
					}
					return nil
				} else {
					eModel.StartDate = result.Time.UTC().Unix()
				}
			case "end":
				if result, err := as.When.Parse(v, time.Now()); err != nil {
					msg := fmt.Sprintf("Can't parse end date\n```\n%s\n```", err.Error())
					if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content: &msg,
					}); err != nil {
						slog.Error("can't respond", "handler", "create-event", "content", "parse-date-error", "error", err)
					}
					return nil
				} else {
					eModel.EndDate = result.Time.UTC().Unix()
				}
			case "location":
				eModel.Location = v
			case "url":
				eModel.URL = v
			case "invitees":
				rawString := v
				for _, invitee := range strings.Split(rawString, ",") {
					invitees = append(invitees, strings.TrimSpace(invitee))
				}
			}
		}
		// #endregion

		// #region | ask to confirm
		yesCustomId := "yes-" + eModel.ID
		cancelCustomId := "cancel-" + eModel.ID
		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)

		// prepare the embed message to ask for confirmation
		fields := []*discordgo.MessageEmbedField{
			{
				Name:   "Start Date",
				Value:  fmt.Sprintf("<t:%d:f>", eModel.StartDate),
				Inline: true,
			},
			{
				Name:   "End Date",
				Value:  fmt.Sprintf("<t:%d:f>", eModel.EndDate),
				Inline: true,
			},
		}
		if eModel.Location != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Location",
				Value: eModel.Location,
			})
		}
		if len(invitees) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Invitees",
				Value: strings.Join(invitees, ", "),
			})
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Is this correct?",
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       eModel.Summary,
						Description: eModel.Description,
						URL:         eModel.URL,
						Fields:      fields,
						Author: &discordgo.MessageEmbedAuthor{
							Name: eModel.Organizer,
						},
					},
				},
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
		}); err != nil {
			slog.Error("can't respond", "handler", "create-event", "content", "confirm-event", "error", err)
		}
		var buttonInteraction *discordgo.Interaction
		as.AddAppCmdHandler(yesCustomId, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			buttonInteraction = i.Interaction
			confirmCh <- struct{}{}
			return nil
		})
		as.AddAppCmdHandler(cancelCustomId, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			buttonInteraction = i.Interaction
			cancelCh <- struct{}{}
			return nil
		})
		defer as.RemoveAppCmdHandler(yesCustomId)
		defer as.RemoveAppCmdHandler(cancelCustomId)

		var resultErr error
		select {
		case <-cancelCh:
			resultErr = fmt.Errorf("event creation canceled")
		case <-confirmCh:
			// upsert to db
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				// main model
				if err := eModel.Upsert(ctx, tx); err != nil {
					return fmt.Errorf("createEventHandler: %w", err)
				}

				// attendee models
				attendeeModels := make([]model.Attendee, len(invitees))
				for i, invitee := range invitees {
					attendeeModels[i] = model.Attendee{
						EventID: eModel.ID,
						Data:    invitee,
					}
				}
				if _, err := tx.NewInsert().
					Model(&attendeeModels).
					Exec(ctx); err != nil {
					return fmt.Errorf("can't insert invitees: %w", err)
				}
				return nil
			}); err != nil {
				resultErr = fmt.Errorf("createEventHandler: %w", err)
			}
		case <-time.After(time.Minute * 2):
			s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation.")
		}

		var msg string
		if resultErr != nil {
			msg = fmt.Sprintf("Event was not created\n```%s```", resultErr.Error())
		} else {
			msg = "Event created."
		}
		if err := s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msg,
			},
		}); err != nil {
			slog.Error("can't respond", "handler", "create-event", "content", "create-event-error", "error", err)
		}

		if resultErr != nil {
			return fmt.Errorf("createEventHandler: %w", resultErr)
		}
		return nil
	}
}
