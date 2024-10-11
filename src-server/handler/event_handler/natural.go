package event_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func natural(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "natural"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Execute CRUD operations on events using natural language.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "content",
				Description: "What do you want to do?",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-context-id",
				Description: "The ID of the event using as context for the LLM.",
				Required:    false,
			},
		},
	})
	cmdHandler[id] = naturalHandler(as)
}

func naturalHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction
		if err := ensureCalendarExists(as, s, i); err != nil {
			return err
		}

		// respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("naturalHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// #region - get user params
		content, eventContextID := func() (string, string) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var content, eventContextID string
			if opt, ok := optionMap["content"]; ok {
				content = strings.TrimSpace(opt.StringValue())
			}
			if opt, ok := optionMap["event-context-id"]; ok {
				eventContextID = opt.StringValue()
			}
			return content, eventContextID
		}()
		if content == "" {
			// edit the deferred message
			msg := "Event content is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about event content is empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the event from database if provided event ID
		naturalInputEventContext := new(utils.NaturalInputEventContext)
		if eventContextID != "" {
			eventModel := new(model.Event)
			exists, err := as.BunDB.NewSelect().
				Model((*model.Event)(nil)).
				Where("id = ?", eventContextID).
				Exists(context.Background())
			if err != nil {
				// edit the deferred message
				msg := fmt.Sprintf("Can't check if event exists\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("naturalHandler: can't respond about can't check if event exists", "error", err)
				}
				return fmt.Errorf("naturalHandler: can't check if event exists: %w", err)
			}
			if !exists {
				// edit the deferred message
				msg := "Event with the provided event ID not found."
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("naturalHandler: can't respond about event not found", "error", err)
				}
				return nil
			}
			if err := as.BunDB.NewSelect().
				Model(eventModel).
				Where("id = ?", eventContextID).
				Relation("Attendees").
				Scan(context.Background()); err != nil {
				slog.Warn("naturalHandler: can't scan event context", "error", err)
			}
			startTime := time.Unix(0, eventModel.StartDateUnixUTC).UTC()
			startTimeStr := startTime.In(time.Now().Location()).Format("02/01/2006 15:04")
			endTime := time.Unix(0, eventModel.EndDateUnixUTC).UTC()
			endTimeStr := endTime.In(time.Now().Location()).Format("02/01/2006 15:04")
			attendees := make([]string, len(eventModel.Attendees))
			for i, attendee := range eventModel.Attendees {
				attendees[i] = attendee.Data
			}
			naturalInputEventContext = &utils.NaturalInputEventContext{
				Title:       eventModel.Summary,
				Description: eventModel.Description,
				Start:       startTimeStr,
				End:         endTimeStr,
				Location:    eventModel.Location,
				URL:         eventModel.URL,
				Attendees:   attendees,
			}
		}
		// #endregion

		// #region - process & check success
		naturalOutput, err := as.Natural.NewRequest(content, naturalInputEventContext)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't perform natural request\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't perform natural request", "error", err)
			}
			return fmt.Errorf("naturalHandler: can't perform natural request: %w", err)
		}
		if !naturalOutput.Success {
			// edit the deferred message
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &naturalOutput.Description,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't perform natural request", "error", err)
			}
			return nil
		}
		// #endregion

		switch naturalOutput.Action {
		case utils.NaturalOutputActionTypeCreate:
			return handleActionTypeCreate(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeRead:
			return handleActionTypeRead(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeUpdate:
			return handleActionTypeUpdate(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeDelete:
			return handleActionTypeDelete(as, s, i, naturalInputEventContext, naturalOutput)
		default:
			// edit the deferred message
			msg := fmt.Sprintf("Can't create event, invalid action %s from the LLM.", naturalOutput.Action)
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't create event, invalid action from the LLM", "error", err)
			}
			return nil
		}
	}
}

func handleActionTypeCreate(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate, naturalOutput utils.NaturalOutput) error {
	interaction := i.Interaction

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
			slog.Warn("naturalHandler: can't respond about can't create event, invalid start date", "error", err)
		}
		return nil
	}
	endDate, err := time.ParseInLocation("02/01/2006 15:04", body.End, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't create event, invalid end date: %s", err.Error())
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, invalid end date", "error", err)
		}
		return nil
	}
	if startDate.After(endDate) {
		// edit the deferred message
		msg := "Can't create event, start date must be before end date."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about can't create event, start date must be before end date", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - init new event model
	newEventModel := model.Event{
		ID:               uuid.NewString(),
		Summary:          body.Title,
		Description:      body.Description,
		Location:         body.Location,
		URL:              body.URL,
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
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
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
	switch {
	case err != nil:
		return fmt.Errorf("naturalHandler -> handleActionTypeCreate: %w", err)
	case timeout:
		// edit ask confirmation message
		msg := "Timed out waiting for confirmation."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("naturalHandler: can't respond about event creation timed out", "error", err)
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
			slog.Warn("naturalHandler: can't edit ask for confirmation message to disable buttons", "error", err)
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
	}); err != nil {
		slog.Warn("naturalHandler: can't edit ask for confirmation message to disable buttons", "error", err)
	}
	// #endregion

	return nil
}
