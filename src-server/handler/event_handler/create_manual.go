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

func createManual(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "create-manual"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Create an event manually.",
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
	cmdHandler[id] = createManualHandler(as)
}

func createManualHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction
		eventID := uuid.New().String()

		// collect data
		staticEvent, err := func() (*model.StaticEvent, error) {
			staticEvent := new(model.StaticEvent)
			staticEvent.Attendees = new([]string)
			staticEvent.ID = eventID

			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if value, ok := optionMap["title"]; ok {
				staticEvent.Title = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["description"]; ok {
				staticEvent.Description = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["start"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse start date: %w", err)
				}
				staticEvent.StartDate = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["end"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse end date: %w", err)
				}
				staticEvent.EndDate = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["location"]; ok {
				staticEvent.Location = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["url"]; ok {
				staticEvent.URL = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["invitees"]; ok {
				rawString := value.StringValue()
				for _, attendee := range strings.Split(rawString, ",") {
					*staticEvent.Attendees = append(*staticEvent.Attendees, strings.TrimSpace(attendee))
				}
			}
			if value, ok := optionMap["whole-day"]; ok {
				staticEvent.IsWholeDay = value.BoolValue()
				startDate := time.Unix(staticEvent.StartDate, 0)
				endDate := time.Unix(staticEvent.EndDate, 0)
				if staticEvent.IsWholeDay {
					startDate = startDate.Truncate(24 * time.Hour)
					endDate = endDate.Truncate(24 * time.Hour)
				}
				staticEvent.StartDate = startDate.UTC().Unix()
				staticEvent.EndDate = endDate.UTC().Unix()
			}

			return staticEvent, nil
		}()
		if err != nil {
			// respond to original request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't create event\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-event", "content", "create-event-error", "error", err)
			}
			return nil
		}

		// static event -> event model
		eventModel := func() *model.MasterEvent {
			eventModel := new(model.MasterEvent)
			eventModel.ID = staticEvent.ID
			eventModel.Organizer = i.Member.User.Username
			eventModel.CalendarID = i.GuildID

			eventModel.Summary = staticEvent.Title
			eventModel.Description = staticEvent.Description
			eventModel.Location = staticEvent.Location
			startDate, endDate := func() (int64, int64) {
				startDate := time.Unix(staticEvent.StartDate, 0)
				endDate := time.Unix(staticEvent.EndDate, 0)
				if staticEvent.IsWholeDay {
					startDate = startDate.Truncate(24 * time.Hour)
					endDate = endDate.Truncate(24 * time.Hour)
				}
				return startDate.Unix(), endDate.Unix()
			}()
			eventModel.StartDate = startDate
			eventModel.EndDate = endDate
			eventModel.URL = staticEvent.URL

			return eventModel
		}()

		// ask for confirmation
		if isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventModel.ID
			cancelCustomId := "cancel-" + eventModel.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Is this correct?",
					Embeds:  eventModel.ToDiscordEmbed(context.Background(), as.BunDB),
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
				if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
					slog.Warn("can't respond", "handler", "create-event", "content", "timed-out", "error", err)
				}
				return false, true, nil
			case <-cancelCh:
				if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Event creation canceled.",
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "create-event", "content", "create-event-canceled", "error", err)
				}
				return false, false, nil
			case <-confirmCh:
				return true, false, nil
			}
		}(); err != nil {
			// respond to original request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't create event\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-event", "content", "create-event-error", "error", err)
			}
			return fmt.Errorf("can't create static event: %w", err)
		} else if timeout {
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("can't respond", "handler", "create-event", "content", "timed-out", "error", err)
			}
			return nil
		} else if !isContinue {
			msg := "Event creation canceled."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-event", "content", "create-event-canceled", "error", err)
			}
			return nil
		}

		// insert to db
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			eventModel.CreatedAt = time.Now().UTC().Unix()
			if err := eventModel.Upsert(ctx, tx); err != nil {
				return err
			}
			attendeeModels := make([]model.Attendee, len(*staticEvent.Attendees))
			for i, invitee := range *staticEvent.Attendees {
				attendeeModels[i] = model.Attendee{
					EventID: eventModel.ID,
					Data:    invitee,
				}
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
				slog.Warn("can't respond", "handler", "create-event", "content", "insert-event-error", "error", err)
			}
			return fmt.Errorf("can't insert event: %w", err)
		}

		// respond to button
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event created.",
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "create-event", "content", "last-msg", "error", err)
		}

		return nil
	}
}
