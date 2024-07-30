package event_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func modify(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "event-modify"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Modify an existing calendar event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "The ID of the event to modify",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "title",
				Description: "The title of the event.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "description",
				Description: "Describe the event in detail.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start",
				Description: "The start date of the event.",
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
	cmdHandler[id] = modifyHandler(as)
}

func modifyHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to original request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			return fmt.Errorf("can't respond deferring msg, can't continue: %w", err)
		}

		// get new event data
		newEvent, err := func() (*model.StaticEvent, error) {
			newEvent := new(model.StaticEvent)
			newEvent.Attendees = new([]string)

			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if value, ok := optionMap["event-id"]; ok {
				newEvent.ID = value.StringValue()
			}
			if value, ok := optionMap["title"]; ok {
				newEvent.Title = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["description"]; ok {
				newEvent.Description = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["start"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse start date: %w", err)
				}
				newEvent.StartDate = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["end"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse end date: %w", err)
				}
				newEvent.EndDate = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["location"]; ok {
				newEvent.Location = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["url"]; ok {
				if _, err := url.ParseRequestURI(value.StringValue()); err != nil {
					return nil, fmt.Errorf("invalid URL: %w", err)
				}
				newEvent.URL = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["invitees"]; ok {
				rawString := value.StringValue()
				for _, attendee := range strings.Split(rawString, ",") {
					*newEvent.Attendees = append(*newEvent.Attendees, strings.TrimSpace(attendee))
				}
			}
			if value, ok := optionMap["whole-day"]; ok {
				newEvent.IsWholeDay = value.BoolValue()
				startDate := time.Unix(newEvent.StartDate, 0)
				endDate := time.Unix(newEvent.EndDate, 0)
				if newEvent.IsWholeDay {
					startDate = startDate.Truncate(24 * time.Hour)
					endDate = endDate.Truncate(24 * time.Hour)
				}
				newEvent.StartDate = startDate.UTC().Unix()
				newEvent.EndDate = endDate.UTC().Unix()
			}

			return newEvent, nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Invalid event data\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "can't-get-new-event", "error", err)
			}
			return nil
		}

		// is modifying a master/child event? | is event exists?
		oldEvent, exist, err := func() (interface{}, bool, error) {
			switch regexp.MustCompile(`^[0-9]+$`).MatchString(newEvent.ID) {
			case true: // child event
				exist, err := as.BunDB.
					NewSelect().
					Model((*model.ChildEvent)(nil)).
					Where("recurrence_id = ?", newEvent.ID).
					Exists(context.Background())
				if err != nil {
					return false, false, fmt.Errorf("can't check if event exists: %w", err)
				}
				if !exist {
					return false, false, nil
				}
				eventModel := new(model.ChildEvent)
				if err := as.BunDB.
					NewSelect().
					Model(eventModel).
					Where("recurrence_id = ?", newEvent.ID).
					Scan(context.Background()); err != nil {
					return nil, false, err
				}
				return eventModel, exist, nil
			default: // master event
				exist, err := as.BunDB.
					NewSelect().
					Model((*model.MasterEvent)(nil)).
					Where("id = ?", newEvent.ID).
					Exists(context.Background())
				if err != nil {
					return false, false, fmt.Errorf("can't check if event exists: %w", err)
				}
				if !exist {
					return false, false, nil
				}
				eventModel := new(model.MasterEvent)
				if err := as.BunDB.
					NewSelect().
					Model(eventModel).
					Where("id = ?", newEvent.ID).
					Scan(context.Background()); err != nil {
					return nil, false, err
				}
				return eventModel, exist, nil
			}
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't check if event exists\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "event-not-found", "error", err)
			}
			return err
		} else if !exist {
			// edit the deferred message
			msg := "Event not found."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "event-not-found", "error", err)
			}
			return nil
		}

		// ask for confirmation
		if isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + newEvent.ID
			cancelCustomId := "cancel-" + newEvent.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// these variables are only for the embed msg
			var title string
			description := "`None` `[unchanged]`"
			startDate := "`None` `[unchanged]`"
			endDate := "`None` `[unchanged]`"
			url := "`None` `[unchanged]`"
			location := "`None` `[unchanged]`"
			attendees := "`None` `[unchanged]`"
			switch oldEvent := oldEvent.(type) {
			case *model.MasterEvent:
				switch newExist, oldExist := newEvent.Title != "", oldEvent.Summary != ""; {
				case newExist && oldExist:
					title = fmt.Sprintf("%s `[old value: %s]`", newEvent.Title, oldEvent.Summary)
				case newExist && !oldExist:
					title = fmt.Sprintf("%s `[old value: None]`", newEvent.Title)
				case !newExist && oldExist:
					title = fmt.Sprintf("%s `[unchanged]`", oldEvent.Summary)
				}

				switch newExist, oldExist := newEvent.Description != "", oldEvent.Description != ""; {
				case newExist && oldExist:
					description = fmt.Sprintf("%s `[old value: %s]`", newEvent.Description, oldEvent.Description)
				case newExist && !oldExist:
					description = fmt.Sprintf("%s `[old value: None]`", newEvent.Description)
				case !newExist && oldExist:
					description = fmt.Sprintf("%s `[unchanged]`", oldEvent.Description)
				}

				switch newExist, oldExist := newEvent.StartDate != 0, oldEvent.StartDate != 0; {
				case newExist && oldExist:
					startDate = fmt.Sprintf("<t:%d:f> `[old value: <t:%d:f>]`", newEvent.StartDate, oldEvent.StartDate)
				case newExist && !oldExist:
					startDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEvent.StartDate)
				case !newExist && oldExist:
					startDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.StartDate)
				}

				switch newExist, oldExist := newEvent.EndDate != 0, oldEvent.EndDate != 0; {
				case newExist && oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[old value: <t:%d:f>]`", newEvent.EndDate, oldEvent.EndDate)
				case newExist && !oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEvent.EndDate)
				case !newExist && oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.EndDate)
				}

				switch newExist, oldExist := newEvent.URL != "", oldEvent.URL != ""; {
				case newExist && oldExist:
					url = fmt.Sprintf("%s `[old value: %s]`", newEvent.URL, oldEvent.URL)
				case newExist && !oldExist:
					url = fmt.Sprintf("%s `[old value: None]`", newEvent.URL)
				case !newExist && oldExist:
					url = fmt.Sprintf("%s `[unchanged]`", oldEvent.URL)
				}

				switch newExist, oldExist := newEvent.Location != "", oldEvent.Location != ""; {
				case newExist && oldExist:
					location = fmt.Sprintf("%s `[old value: %s]`", newEvent.Location, oldEvent.Location)
				case newExist && !oldExist:
					location = fmt.Sprintf("%s `[old value: None]`", newEvent.Location)
				case !newExist && oldExist:
					location = fmt.Sprintf("%s `[unchanged]`", oldEvent.Location)
				}

				oldAttendees := func() string {
					var attendeeModels []model.Attendee
					if err := as.BunDB.
						NewSelect().
						Model(&attendeeModels).
						Where("event_id = ?", oldEvent.ID).
						Scan(context.Background()); err != nil {
						slog.Warn("can't get attendees", "handler", "modify-event", "purpose", "display-changes", "error", err)
						return ""
					}
					attendees := make([]string, len(attendeeModels))
					for i, attendee := range attendeeModels {
						attendees[i] = attendee.Data
					}
					return strings.Join(attendees, ", ")
				}()
				newAttendees := strings.Join(*newEvent.Attendees, ", ")
				switch newExist, oldExist := newAttendees != "", oldAttendees != ""; {
				case newExist && oldExist:
					attendees = fmt.Sprintf("%s `[old value: %s]`", newAttendees, oldAttendees)
				case newExist && !oldExist:
					attendees = fmt.Sprintf("%s `[old value: None]`", newAttendees)
				case !newExist && oldExist:
					attendees = fmt.Sprintf("%s `[unchanged]`", oldAttendees)
				}
			case *model.ChildEvent:
				switch newExist, oldExist := newEvent.Title != "", oldEvent.Summary != ""; {
				case newExist && oldExist:
					title = fmt.Sprintf("%s `[old value: %s]`", newEvent.Title, oldEvent.Summary)
				case newExist && !oldExist:
					title = fmt.Sprintf("%s `[old value: None]`", newEvent.Title)
				case !newExist && oldExist:
					title = fmt.Sprintf("%s `[unchanged]`", oldEvent.Summary)
				}

				switch newExist, oldExist := newEvent.Description != "", oldEvent.Description != ""; {
				case newExist && oldExist:
					description = fmt.Sprintf("%s `[old value: %s]`", newEvent.Description, oldEvent.Description)
				case newExist && !oldExist:
					description = fmt.Sprintf("%s `[old value: None]`", newEvent.Description)
				case !newExist && oldExist:
					description = fmt.Sprintf("%s `[unchanged]`", oldEvent.Description)
				}

				switch newExist, oldExist := newEvent.StartDate != 0, oldEvent.StartDate != 0; {
				case newExist && oldExist:
					startDate = fmt.Sprintf("<t:%d:f> [old value: <t:%d:f>]", newEvent.StartDate, oldEvent.StartDate)
				case newExist && !oldExist:
					startDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEvent.StartDate)
				case !newExist && oldExist:
					startDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.StartDate)
				}

				switch newExist, oldExist := newEvent.EndDate != 0, oldEvent.EndDate != 0; {
				case newExist && oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[old value: <t:%d:f>]`", newEvent.EndDate, oldEvent.EndDate)
				case newExist && !oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEvent.EndDate)
				case !newExist && oldExist:
					endDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.EndDate)
				}

				switch newExist, oldExist := newEvent.URL != "", oldEvent.URL != ""; {
				case newExist && oldExist:
					url = fmt.Sprintf("%s `[old value: %s]`", newEvent.URL, oldEvent.URL)
				case newExist && !oldExist:
					url = fmt.Sprintf("%s `[old value: None]`", newEvent.URL)
				case !newExist && oldExist:
					url = fmt.Sprintf("%s `[unchanged]`", oldEvent.URL)
				}

				switch newExist, oldExist := newEvent.Location != "", oldEvent.Location != ""; {
				case newExist && oldExist:
					location = fmt.Sprintf("%s `[old value: %s]`", newEvent.Location, oldEvent.Location)
				case newExist && !oldExist:
					location = fmt.Sprintf("%s `[old value: None]`", newEvent.Location)
				case !newExist && oldExist:
					location = fmt.Sprintf("%s `[unchanged]`", oldEvent.Location)
				}

				oldAttendees := func() string {
					var attendeeModels []model.Attendee
					if err := as.BunDB.
						NewSelect().
						Model(&attendeeModels).
						Where("event_id = ?", oldEvent.RecurrenceID).
						Scan(context.Background()); err != nil {
						slog.Warn("can't get attendees", "handler", "modify-event", "purpose", "display-changes", "error", err)
						return ""
					}
					attendees := make([]string, len(attendeeModels))
					for i, attendee := range attendeeModels {
						attendees[i] = attendee.Data
					}
					return strings.Join(attendees, ", ")
				}()
				newAttendees := strings.Join(*newEvent.Attendees, ", ")
				switch newExist, oldExist := newAttendees != "", oldAttendees != ""; {
				case newExist && oldExist:
					attendees = fmt.Sprintf("%s `[old value: %s]`", newAttendees, oldAttendees)
				case newExist && !oldExist:
					attendees = fmt.Sprintf("%s `[old value: None]`", newAttendees)
				case !newExist && oldExist:
					attendees = fmt.Sprintf("%s `[unchanged]`", oldAttendees)
				}
			}

			msg := "Is this correct?"
			// edit the deferred message
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds: &[]*discordgo.MessageEmbed{
					{
						Title:       title,
						Description: description,
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:   "Start Date",
								Value:  startDate,
								Inline: true,
							},
							{
								Name:   "End Date",
								Value:  endDate,
								Inline: true,
							},
							{
								Name:  "Location",
								Value: location,
							},
							{
								Name:  "URL",
								Value: url,
							},
							{
								Name:  "Attendees",
								Value: attendees,
							},
						},
						Footer: &discordgo.MessageEmbedFooter{
							Text: newEvent.ID,
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
		}(); err != nil {
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't update event\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "can't-ask-confirm", "error", err)
			}
			return err
		} else if timeout {
			// respond to nothing
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "timed-out", "error", err)
			}
			return nil
		} else if !isContinue {
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event not modified.",
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "cancel", "error", err)
			}
			return nil
		}

		// update event
		switch oldEvent := oldEvent.(type) {
		case *model.MasterEvent:
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				if newEvent.Title != "" {
					oldEvent.Summary = newEvent.Title
				}
				if newEvent.Description != "" {
					oldEvent.Description = newEvent.Description
				}
				if newEvent.StartDate != 0 {
					oldEvent.StartDate = newEvent.StartDate
				}
				if newEvent.EndDate != 0 {
					oldEvent.EndDate = newEvent.EndDate
				}
				if newEvent.Location != "" {
					oldEvent.Location = newEvent.Location
				}
				if newEvent.URL != "" {
					oldEvent.URL = newEvent.URL
				}
				oldEvent.UpdatedAt = time.Now().UTC().Unix()
				if err := oldEvent.Upsert(ctx, tx); err != nil {
					return err
				}

				if len(*newEvent.Attendees) > 0 {
					if _, err := tx.
						NewDelete().
						Model((*model.Attendee)(nil)).
						Where("event_id = ?", newEvent.ID).
						Exec(ctx); err != nil {
						return err
					}
					if _, err := tx.NewInsert().
						Model(func() *[]model.Attendee {
							attendeeModels := make([]model.Attendee, len(*newEvent.Attendees))
							for _, attendee := range *newEvent.Attendees {
								attendeeModel := new(model.Attendee)
								attendeeModel.EventID = oldEvent.ID
								attendeeModel.Data = attendee
								attendeeModels = append(attendeeModels, *attendeeModel)
							}
							return &attendeeModels
						}()).
						Exec(ctx); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				// respond to button request
				if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Can't update event\n```%s```", err.Error()),
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "modify-event", "content", "event-update-error", "error", err)
				}
				return err
			}
		case *model.ChildEvent:
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				if newEvent.Title != "" {
					oldEvent.Summary = newEvent.Title
				}
				if newEvent.Description != "" {
					oldEvent.Description = newEvent.Description
				}
				if newEvent.StartDate != 0 {
					oldEvent.StartDate = newEvent.StartDate
				}
				if newEvent.EndDate != 0 {
					oldEvent.EndDate = newEvent.EndDate
				}
				if newEvent.Location != "" {
					oldEvent.Location = newEvent.Location
				}
				if newEvent.URL != "" {
					oldEvent.URL = newEvent.URL
				}
				if err := oldEvent.Upsert(ctx, tx); err != nil {
					return err
				}

				if len(*newEvent.Attendees) > 0 {
					if _, err := tx.
						NewDelete().
						Model((*model.Attendee)(nil)).
						Where("event_id = ?", oldEvent.ID).
						Exec(ctx); err != nil {
						return err
					}
					if _, err := tx.NewInsert().
						Model(func() *[]model.Attendee {
							var attendeeModels []model.Attendee
							for _, attendee := range *newEvent.Attendees {
								attendeeModel := new(model.Attendee)
								attendeeModel.EventID = oldEvent.ID
								attendeeModel.Data = attendee
								attendeeModels = append(attendeeModels, *attendeeModel)
							}
							return &attendeeModels
						}()).
						Exec(ctx); err != nil {
						return err
					}
				}
				return nil

			}); err != nil {
				if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Can't update event\n```%s```", err.Error()),
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "modify-event", "content", "event-update-error", "error", err)
				}
				return err
			}
		}

		// respond to button request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event updated.",
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "modify-event", "content", "event-update-success", "error", err)
		}

		return nil
	}
}
