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

func handleActionTypeDelete(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate, naturalInputEventModel *model.Event) error {
	// #region - validate
	if naturalInputEventModel.ChannelID != naturalInputEventModel.CalendarID {
		// edit the deferred message
		msg := "You cannot delete events in external calendars."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:delete: can't respond about can't create event, invalid event context", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - ask for confirmation
	askForConfirmInteraction := i.Interaction
	buttonInteraction := new(discordgo.Interaction)
	isContinue, timeout, err := func() (bool, bool, error) {
		yesCustomId := "yes-" + naturalInputEventModel.ID
		cancelCustomId := "cancel-" + naturalInputEventModel.ID
		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)

		msg := "Is this the event you want to delete?"
		// edit the deferred message
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Embeds:  &[]*discordgo.MessageEmbed{naturalInputEventModel.ToDiscordEmbed()},
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
			slog.Warn("event_handler:natural:delete: can't defer ask for confirmation message", "error", err)
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
		slog.Warn("event_handler:natural:delete: can't edit ask for confirmation message to disable buttons", "error", err)
	}
	// #endregion

	// #region - handle ask-for-confirm cases
	switch {
	case err != nil:
		return fmt.Errorf("event_handler:natural:delete -> handleActionTypeCreate: %w", err)
	case timeout:
		// edit ask-for-confirm msg
		msg := "Timed out waiting for confirmation."
		if _, err := s.InteractionResponseEdit(askForConfirmInteraction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:delete: can't respond about event creation timed out", "error", err)
		}
		return nil
	case !isContinue:
		// edit deferred response of button click
		msg := "Event creation canceled."
		if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:delete: can't respond about event creation canceled", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - delete event
	if _, err := as.BunDB.NewDelete().
		Model((*model.Event)(nil)).
		Where("id = ?", naturalInputEventModel.ID).
		Where("channel_id = ?", i.Interaction.ChannelID).
		Exec(context.WithValue(context.Background(), model.EventIDCtxKey, naturalInputEventModel.ID)); err != nil {
		// edit deferred response of button click
		msg := fmt.Sprintf("Can't delete event\n```\n%s\n```", err.Error())
		if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:delete: can't respond about can't delete event", "error", err)
		}
		return fmt.Errorf("event_handler:natural:delete: can't delete event: %w", err)
	}
	// #endregion

	// #region - edit deferred response of button click
	msg := "Event deleted."
	if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
		Content: &msg,
	}); err != nil {
		slog.Warn("event_handler:natural:delete: can't edit event deletion success message", "error", err)
	}
	// #endregion

	return nil
}
