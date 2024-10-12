package event_handler

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
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func create(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "create"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Create an event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "title",
				Description: "The title of the event.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start",
				Description: "The start date of the event.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "description",
				Description: "Describe the event in detail.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end",
				Description: "The end date of the event.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "whole-day",
				Description: "Is the event a whole day event?",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "location",
				Description: "The location of the event.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "The URL of the event.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "invitees",
				Description: "List the invitees of the event, each separated by a comma.",
				Required:    false,
			},
		},
	})
	cmdHandler[id] = createHandler(as)
}

func createHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := ensureCalendarExists(as, s, i); err != nil {
			return fmt.Errorf("event_handler:create: can't ensure calendar exists: %w", err)
		}

		// #region - reply w/ deferred
		startTimer := time.Now()
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("event_handler:create: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// #region - parse user params
		var attendeeModels []model.Attendee
		eventModel, err := func() (*model.Event, error) {
			eventModel := new(model.Event)
			eventModel.ID = uuid.NewString()
			eventModel.CalendarID = i.ChannelID
			eventModel.ChannelID = i.ChannelID
			eventModel.NotificationSent = false
			if i.Member != nil && i.Member.User != nil {
				eventModel.Organizer = i.Member.User.Username
			}

			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if value, ok := optionMap["title"]; ok {
				eventModel.Summary = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["description"]; ok {
				eventModel.Description = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["location"]; ok {
				eventModel.Location = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["url"]; ok {
				eventModel.URL = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["start"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse start date: %w", err)
				}
				eventModel.StartDateUnixUTC = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["end"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse end date: %w", err)
				}
				eventModel.EndDateUnixUTC = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["invitees"]; ok {
				rawString := value.StringValue()
				for _, attendee := range strings.Split(rawString, ",") {
					attendee := strings.TrimSpace(attendee)
					if attendee != "" {
						attendeeModels = append(attendeeModels, model.Attendee{
							EventID: eventModel.ID,
							Data:    attendee,
						})
					}
				}
			}
			if value, ok := optionMap["whole-day"]; ok {
				eventModel.IsWholeDay = value.BoolValue()
				startDate := time.Unix(eventModel.StartDateUnixUTC, 0)
				endDate := time.Unix(eventModel.EndDateUnixUTC, 0)
				if eventModel.IsWholeDay {
					startDate = startDate.Truncate(24 * time.Hour)
					endDate = endDate.Truncate(24 * time.Hour)
				}
				eventModel.StartDateUnixUTC = startDate.UTC().Unix()
				eventModel.EndDateUnixUTC = endDate.UTC().Unix()
			}

			return eventModel, nil
		}()
		if err != nil {
			// respond to original request
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Invalid data provided\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("event_handler:create: can't respond about invalid data provided", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - ask for confirmation
		askForConfirmInteraction := interaction
		isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventModel.ID
			cancelCustomId := "cancel-" + eventModel.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// edit the deferred message
			msg := "Is this correct?"
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds:  &[]*discordgo.MessageEmbed{eventModel.ToDiscordEmbed(context.Background(), as.BunDB)},
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
		// #endregion

		// #region - reply to ask-for-confirm w/ deferred
		if err := s.InteractionRespond(
			interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			},
		); err != nil {
			slog.Warn("event_handler:create: can't defer ask for confirmation message", "error", err)
		}

		// edit ask for confirmation message to disable buttons
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
			slog.Warn("event_handler:create: can't edit ask for confirmation message to disable buttons", "error", err)
		}
		// #endregion

		// #region - handle ask-for-confirm cases
		switch {
		case err != nil:
			return fmt.Errorf("event_handler::createHandler: %w", err)
		case timeout:
			// edit ask for confirmation message
			msg := "Timed out waiting for confirmation."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content:    &msg,
				Components: &[]discordgo.MessageComponent{},
			}); err != nil {
				slog.Warn("createEventManualHandler: can't respond about event creation timed out", "error", err)
			}
			return nil
		case !isContinue:
			// response ask confirmation message
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event creation canceled.",
				},
			}); err != nil {
				slog.Warn("createEventManualHandler: can't respond about event creation canceled", "error", err)
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
				slog.Warn("createEventManualHandler: can't edit ask for confirmation message to disable buttons", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - insert to db
		startTimer = time.Now()
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			eventModel.CreatedAt = time.Now().UTC().Unix()
			if err := eventModel.Upsert(ctx, tx); err != nil {
				return err
			}
			if _, err := tx.NewInsert().
				Model(&attendeeModels).
				Exec(ctx); err != nil {
				return err
			}
			return nil
		}); err != nil {
			// respond to button
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't insert event to database\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("createEventManualHandler: can't respond about can't insert event to database", "error", err)
			}
			return fmt.Errorf("createEventManualHandler: can't insert event to database: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// respond to button
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event created.",
			},
		}); err != nil {
			slog.Warn("createEventManualHandler: can't respond about event creation success", "error", err)
		}
		// edit ask for confirmation message to disable buttons
		// #region - edit the deferred response of the button
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
			slog.Warn("event_handler:create: can't edit deferred response of the button click", "error", err)
		}
		// #endregion

		return nil
	}
}
