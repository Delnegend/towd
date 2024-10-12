package event_handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func delete(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "delete"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Delete an event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "ID of the event to delete.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = deleteHandler(as)
}

func deleteHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		// #region - respond to original request
		startTimer := time.Now()
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("event_handler:delete: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// #region - get the event ID
		eventID := func() string {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var eventID string
			if opt, ok := optionMap["event-id"]; ok {
				eventID = opt.StringValue()
			}
			return eventID
		}()
		if eventID == "" {
			// edit the deferred message
			msg := "Event ID is empty."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about event ID is empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - check if event exists
		startTimer = time.Now()
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.Event)(nil)).
			Where("id = ?", eventID).
			Exists(context.Background())
		as.MetricChans.DatabaseRead <- float64(time.Since(startTimer).Microseconds())
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't check if event exists\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about can't check if event exists", "error", err)
			}
			return fmt.Errorf("event_handler:delete: can't check if event exists: %w", err)
		case !exists:
			// edit the deferred message
			msg := "Event not found."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about event not found", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the event model
		eventModel := new(model.Event)
		if err := as.BunDB.
			NewSelect().
			Model(eventModel).
			Where("id = ?", eventID).
			Where("channel_id = ?", i.Interaction.ChannelID).
			Scan(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get event\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about can't get event", "error", err)
			}
			return fmt.Errorf("event_handler:delete: can't get event: %w", err)
		}
		// #endregion

		// #region - ask for confirmation
		askForConfirmInteraction := i.Interaction
		buttonInteraction := new(discordgo.Interaction)
		isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventID
			cancelCustomId := "cancel-" + eventID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// edit the deferred message
			msg := "Is this the event you want to delete?"
			if _, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds: &[]*discordgo.MessageEmbed{
					eventModel.ToDiscordEmbed(context.Background(), as.BunDB),
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
								Style:    discordgo.DangerButton,
								CustomID: cancelCustomId,
							},
						},
					},
				},
			}); err != nil {
				return false, false, fmt.Errorf("can't ask for confirmation, can't continue: %w", err)
			}
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

			select {
			case <-time.After(time.Minute * 2):
				return false, true, nil
			case <-cancelCh:
				return false, false, nil
			case <-confirmCh:
				return true, false, nil
			}
		}()
		// #endregion

		// #region - reply buttons click w/ deferred
		if !timeout {
			if err := s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			},
			); err != nil {
				slog.Warn("event_handler:delete: can't defer ask for confirmation message", "error", err)
			}
		}

		// disable ask-for-confirm-msg buttons
		if _, err := s.InteractionResponseEdit(askForConfirmInteraction, &discordgo.WebhookEdit{
			Components: &[]discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes",
							Style:    discordgo.SuccessButton,
							CustomID: "yes-disabled",
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: "cancel-disabled",
							Disabled: true,
						},
					},
				},
			},
		}); err != nil {
			slog.Warn("event_handler:delete: can't edit ask for confirmation message to disable buttons", "error", err)
		}
		// #endregion
		// #region - handle ask-for-confirm cases
		switch {
		case err != nil:
			return fmt.Errorf("event_handler:delete: %w", err)
		case timeout:
			// edit ask confirmation message
			msg := "Timed out waiting for confirmation."
			if _, err := s.InteractionResponseEdit(askForConfirmInteraction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds:  &[]*discordgo.MessageEmbed{},
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about event deletion timed out", "error", err)
			}
			return nil
		case !isContinue:
			// respond ask confirmation message
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event deletion canceled.",
				},
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about event deletion canceled", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - delete event
		startTimer = time.Now()
		if _, err := as.BunDB.NewDelete().
			Model((*model.Event)(nil)).
			Where("id = ?", eventID).
			Where("channel_id = ?", i.Interaction.ChannelID).
			Exec(context.WithValue(context.Background(), model.EventIDCtxKey, eventID)); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't delete event\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("event_handler:delete: can't respond about can't delete event", "error", err)
			}
			return fmt.Errorf("event_handler:delete: can't delete event: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event deleted.",
			},
		}); err != nil {
			slog.Warn("event_handler:delete: can't edit deferred response of the button click", "error", err)
		}
		// #endregion

		return nil
	}
}
