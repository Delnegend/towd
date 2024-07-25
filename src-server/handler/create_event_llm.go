package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func CreateEventLLM(as *utils.AppState) {
	id := "create-event-llm"
	as.AddAppCmdHandler(id, createEventLLMHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Create an event using LLM",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "content",
				Description: "The full content of the event",
				Required:    true,
			},
		},
	})
}

func createEventLLMHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "create-event-llm", "content", "deferring", "error", err)
		}

		// #region | get the content
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(i.ApplicationCommandData().Options))
		for _, opt := range i.ApplicationCommandData().Options {
			optionMap[opt.Name] = opt
		}
		var content string
		if opt, ok := optionMap["content"]; ok {
			content = opt.StringValue()
		}
		if content == "" {
			msg := "Content is required."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-id-required", "error", err)
				return fmt.Errorf("modifyEventHandler: %w", err)
			}
		}
		// #endregion

		// #region | natural text -> static event
		staticEvent := new(model.StaticEvent)
		if err := staticEvent.FromNaturalText(context.Background(), as, content); err != nil {
			msg := fmt.Sprintf("Can't create event\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-create-error", "error", err)
			}
			return fmt.Errorf("createEventLLMHandler: %w", err)
		}
		// #endregion

		// #region | confirm
		yesCustomId := "yes-" + staticEvent.ID
		cancelCustomId := "cancel-" + staticEvent.ID
		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)

		// preparing the embed message to ask for confirmation
		fields := []*discordgo.MessageEmbedField{
			{
				Name:   "Start Date",
				Value:  fmt.Sprintf("<t:%d:f>", staticEvent.StartDate),
				Inline: true,
			},
			{
				Name:   "End Date",
				Value:  fmt.Sprintf("<t:%d:f>", staticEvent.EndDate),
				Inline: true,
			},
		}
		if staticEvent.Location != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Location",
				Value: staticEvent.Location,
			})
		}
		if staticEvent.Attendees != nil {
			if len(*staticEvent.Attendees) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:  "Attendees",
					Value: strings.Join(*staticEvent.Attendees, ", "),
				})
			}
		}
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{
				{
					Title:       staticEvent.Title,
					Description: staticEvent.Description,
					URL:         staticEvent.URL,
					Fields:      fields,
					Footer: &discordgo.MessageEmbedFooter{
						Text: staticEvent.ID,
					},
					Author: &discordgo.MessageEmbedAuthor{
						Name: i.Member.User.Username,
					},
				},
			},
			Components: &[]discordgo.MessageComponent{
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
		}); err != nil {
			slog.Error("can't respond", "handler", "modify-event", "content", "event-create-success", "error", err)
		}

		// handling the buttons
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
			// upsert to db using transaction
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				// main model
				eModel := new(model.MasterEvent)
				eModel.ID = staticEvent.ID
				eModel.Summary = staticEvent.Title
				eModel.Description = staticEvent.Description
				eModel.StartDate = staticEvent.StartDate
				eModel.EndDate = staticEvent.EndDate
				eModel.Location = staticEvent.Location
				eModel.URL = staticEvent.URL
				if err := eModel.Upsert(ctx, tx); err != nil {
					return fmt.Errorf("createEventLLMHandler: %w", err)
				}

				// attendee models
				attendeeModels := make([]model.Attendee, len(*staticEvent.Attendees))
				for i, invitee := range *staticEvent.Attendees {
					attendeeModels[i] = model.Attendee{
						EventID: eModel.ID,
						Data:    invitee,
					}
				}
				if _, err := tx.NewInsert().
					Model(&attendeeModels).
					Exec(ctx); err != nil {
					return fmt.Errorf("creteEventLLMHandler: can't insert attendees: %w", err)
				}
				return nil
			}); err != nil {
				resultErr = fmt.Errorf("createEventLLMHandler: %w", err)
			}
		case <-time.After(time.Minute * 2):
			s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation.")
			return nil
		}
		// #endregion

		var msg string
		if resultErr != nil {
			msg = fmt.Sprintf("Event was not created\n```%s```", resultErr.Error())
		} else {
			msg = "Event created."
		}

		if err := s.InteractionRespond(buttonInteraction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
				},
			},
		); err != nil {
			slog.Error("can't respond", "handler", "modify-event", "error", err)
		}

		if resultErr != nil {
			return fmt.Errorf("createEventLLMHandler: %w", resultErr)
		}
		return nil
	}
}
