package event_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func handleActionTypeUpdate(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate, oldEventModel *model.Event, naturalOutput utils.NaturalOutput) error {
	// #region - validate
	if oldEventModel == nil {
		// edit the deferred message
		msg := "You must provide event context ID to update an event."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid event context from the LLM", "error", err)
		}
		return nil
	}

	if oldEventModel.ChannelID != oldEventModel.CalendarID {
		// edit the deferred message
		msg := "You cannot update events in external calendars."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid event context", "error", err)
		}
		return nil
	}
	if naturalOutput.Body.URL != "" {
		if _, err := url.ParseRequestURI(naturalOutput.Body.URL); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't update event, invalid URL: %s", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't create event, invalid URL", "error", err)
			}
			return nil
		}
	}
	startDate, err := time.ParseInLocation("02/01/2006 15:04", naturalOutput.Body.Start, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't update event, invalid start date: %s", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid start date", "error", err)
		}
		return nil
	}
	startDate = startDate.UTC()
	endDate, err := time.ParseInLocation("02/01/2006 15:04", naturalOutput.Body.End, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't update event, invalid end date: %s", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid end date", "error", err)
		}
		return nil
	}
	endDate = endDate.UTC()
	if startDate.After(endDate) {
		// edit the deferred message
		msg := "Can't update event, start date must be before end date."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, start date must be before end date", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - compose new event
	newEventModel := model.Event{
		ID:               oldEventModel.ID,
		Summary:          naturalOutput.Body.Title,
		Description:      naturalOutput.Body.Description,
		Location:         naturalOutput.Body.Location,
		URL:              naturalOutput.Body.URL,
		Organizer:        oldEventModel.Organizer,
		StartDateUnixUTC: startDate.Unix(),
		EndDateUnixUTC:   endDate.Unix(),
		CreatedAt:        oldEventModel.CreatedAt,
		Sequence:         0,
		CalendarID:       oldEventModel.CalendarID,
		ChannelID:        oldEventModel.ChannelID,
		Attendees: func() []*model.Attendee {
			attendeeModels := make([]*model.Attendee, len(naturalOutput.Body.Attendees))
			for i, attendee := range naturalOutput.Body.Attendees {
				attendeeModels[i] = &model.Attendee{
					EventID: oldEventModel.ID,
					Data:    attendee,
				}
			}
			return attendeeModels
		}(),
		NotificationSent: false,
	}
	// #endregion

	// #region - ask for confirmation
	askForConfirmInteraction := i.Interaction
	buttonInteraction := new(discordgo.Interaction)
	isContinue, timeout, err := func() (bool, bool, error) {
		yesCustomId := "yes-" + newEventModel.ID
		cancelCustomId := "cancel-" + newEventModel.ID
		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)

		oldEvent := new(model.Event)
		if err := as.BunDB.
			NewSelect().
			Model(oldEvent).
			Relation("Attendees").
			Where("id = ?", newEventModel.ID).
			Scan(context.Background()); err != nil {
			return false, false, fmt.Errorf("can't get old event: %w", err)
		}

		diff := oldEvent.Diff(&newEventModel)

		msg := "Is this correct?"
		// edit the deferred message
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Embeds: &[]*discordgo.MessageEmbed{
				{
					Title:       diff.Title,
					Description: diff.Description,
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Start Date",
							Value:  diff.StartDate,
							Inline: true,
						},
						{
							Name:   "End Date",
							Value:  diff.EndDate,
							Inline: true,
						},
						{
							Name:  "Location",
							Value: diff.Location,
						},
						{
							Name:  "URL",
							Value: diff.URL,
						},
						{
							Name:  "Attendees",
							Value: diff.Attendees,
						},
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: newEventModel.ID,
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
							CustomID: yesCustomId,
							Style:    discordgo.SuccessButton,
						},
						discordgo.Button{
							Label:    "Cancel",
							CustomID: cancelCustomId,
							Style:    discordgo.DangerButton,
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
			slog.Warn("event_handler:natural:update: can't defer ask for confirmation message", "error", err)
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
		slog.Warn("event_handler:natural:update: can't edit ask for confirmation message to disable buttons", "error", err)
	}
	// #endregion

	// #region - handle ask-for-confirm cases
	switch {
	case err != nil:
		return fmt.Errorf("event_handler:natural:update: %w", err)
	case timeout:
		// edit ask-for-confirm msg
		msg := "Timed out waiting for confirmation."
		if _, err := s.InteractionResponseEdit(askForConfirmInteraction, &discordgo.WebhookEdit{
			Content:    &msg,
			Components: &[]discordgo.MessageComponent{},
		}); err != nil {
			slog.Warn("event_handler:natural:update: can't respond about event modification timed out", "error", err)
		}
		return nil
	case !isContinue:
		// edit deferred ask-for-confirm button response
		msg := "Event not modified."
		if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:update: can't respond about event modification canceled", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - update event
	if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		newEventModel.UpdatedAt = time.Now().UTC().Unix()
		if err := newEventModel.Upsert(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.
			NewDelete().
			Model((*model.Attendee)(nil)).
			Where("event_id = ?", newEventModel.ID).
			Exec(ctx); err != nil {
			return err
		}
		if len(newEventModel.Attendees) > 0 {
			attendeeModelsDeref := make([]model.Attendee, len(naturalOutput.Body.Attendees))
			for i, model := range newEventModel.Attendees {
				attendeeModelsDeref[i] = *model
			}
			if _, err := tx.NewInsert().Model(&attendeeModelsDeref).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// edit deferred response of button click
		msg := fmt.Sprintf("Can't update event\n```%s```", err.Error())
		if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:update: can't respond about can't update event in database", "error", err)
		}
		return fmt.Errorf("event_handler:natural:update: can't update event in database: %w", err)
	}
	// #endregion

	// #region - edit deferred response of button click
	msg := "Event updated."
	if _, err := s.InteractionResponseEdit(buttonInteraction, &discordgo.WebhookEdit{
		Content: &msg,
	}); err != nil {
		slog.Warn("event_handler:natural:update: can't respond about event update success", "error", err)
	}
	// #endregion

	return nil
}
