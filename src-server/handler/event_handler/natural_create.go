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
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func handleActionTypeCreate(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate, naturalOutput utils.NaturalOutput) error {
	body, ok := naturalOutput.Body.(utils.NaturalOutputBodyForCreateOrUpdate)
	if !ok {
		// edit the deferred message
		msg := "Can't create event, invalid body from the LLM."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid body from the LLM", "error", err)
		}
		return nil
	}

	// #region - validate
	if _, err := url.ParseRequestURI(body.URL); err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't create event, invalid URL: %s", err.Error())
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid URL", "error", err)
		}
		return nil
	}
	startDate, err := time.ParseInLocation("02/01/2006 15:04", body.Start, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't create event, invalid start date: %s", err.Error())
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:create: can't respond about can't create event, invalid start date", "error", err)
		}
		return nil
	}
	endDate, err := time.ParseInLocation("02/01/2006 15:04", naturalOutput.Body.End, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't create event, invalid end date: %s", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:create: can't respond about can't create event, invalid end date", "error", err)
		}
		return nil
	}
	if startDate.After(endDate) {
		// edit the deferred message
		msg := "Can't create event, start date must be before end date."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:create: can't respond about can't create event, start date must be before end date", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - init new event model
	newEventModel := model.Event{
		ID:               uuid.NewString(),
		Summary:          naturalOutput.Body.Title,
		Description:      naturalOutput.Body.Description,
		Location:         naturalOutput.Body.Location,
		URL:              naturalOutput.Body.URL,
		Organizer:        i.Member.User.Username,
		StartDateUnixUTC: startDate.UTC().Unix(),
		EndDateUnixUTC:   endDate.UTC().Unix(),
		CreatedAt:        time.Now().UTC().Unix(),
		Sequence:         0,
		CalendarID:       i.ChannelID,
		ChannelID:        i.ChannelID,
		NotificationSent: false,
	}
	// #endregion

	// #region - ask for confirmation
	isContinue, timeout, err := func() (bool, bool, error) {
		yesCustomId := "yes-" + newEventModel.ID
		cancelCustomId := "cancel-" + newEventModel.ID
		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)

		// edit the deferred message
		msg := "You're creating a new event. Is this correct?"
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Embeds:  &[]*discordgo.MessageEmbed{newEventModel.ToDiscordEmbed(context.Background(), as.BunDB)},
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
			interaction = i.Interaction
			confirmCh <- struct{}{}
			return nil
		})
		as.AddAppCmdHandler(cancelCustomId, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			interaction = i.Interaction
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
	// #region - reply buttons click w/ deferred
	// #endregion

	// #region - handle ask-for-confirm cases
	switch {
	case err != nil:
		return fmt.Errorf("event_handler:natural:create -> handleActionTypeCreate: %w", err)
	case timeout:
		// edit ask confirmation message
		msg := "Timed out waiting for confirmation."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:create: can't respond about event creation timed out", "error", err)
		}
		return nil
	case !isContinue:
		// respond ask confirmation message
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event creation canceled.",
			},
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about event creation canceled", "error", err)
		}
		// edit ask for confirmation message to disable buttons
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Components: &[]discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes",
							Style:    discordgo.SuccessButton,
							CustomID: "",
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: "",
							Disabled: true,
						},
					},
				},
			},
		}); err != nil {
			slog.Warn("event_handler:natural:create: can't respond about event creation canceled", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - insert to DB
	if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().
			Model(&newEventModel).
			Exec(ctx); err != nil {
			return fmt.Errorf("can't insert event to database: %w", err)
		}
		attendeeModels := make([]model.Attendee, len(body.Attendees))
		for i, attendee := range body.Attendees {
			attendeeModels[i] = model.Attendee{
				EventID: newEventModel.ID,
				Data:    attendee,
			}
		}
		if _, err := tx.NewInsert().
			Model(&attendeeModels).
			Exec(ctx); err != nil {
			return fmt.Errorf("can't insert attendees to database: %w", err)
		}
		return nil
	}); err != nil {
		// respond the ask confirmation message
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Can't insert event to database\n```\n" + err.Error() + "\n```",
			},
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't insert event to database", "error", err)
		}
		// edit ask for confirmation message to disable buttons
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Components: &[]discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes",
							Style:    discordgo.SuccessButton,
							CustomID: "",
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: "",
							Disabled: true,
						},
					},
				},
			},
		}); err != nil {
			slog.Warn("naturalHandler: can't edit ask for confirmation message to disable buttons", "error", err)
		}
		return fmt.Errorf("event_handler::naturalHandler: %w", err)
	}
	// #endregion

	// #region - respond ask confirmation message
	if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Event created.",
		},
	}); err != nil {
		slog.Warn("naturalHandler: can't respond about event creation success", "error", err)
	}
	// edit ask for confirmation message to disable buttons
	if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Components: &[]discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Yes",
						Style:    discordgo.SuccessButton,
						CustomID: "",
						Disabled: true,
					},
					discordgo.Button{
						Label:    "Cancel",
						Style:    discordgo.DangerButton,
						CustomID: "",
						Disabled: true,
					},
				},
			},
		},
	// #region - edit deffered response of button click
	}); err != nil {
		slog.Warn("naturalHandler: can't edit ask for confirmation message to disable buttons", "error", err)
	}
	// #endregion

	return nil
}
