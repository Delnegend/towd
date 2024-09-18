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
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			// return fmt.Errorf("can't respond deferring msg, can't continue: %w", err)
			slog.Warn("modifyEventHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// #region - get new event data
		attendeeModels := make([]model.Attendee, 0)
		newEventModel, err := func() (*model.Event, error) {
			newEventModel := new(model.Event)

			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			if value, ok := optionMap["event-id"]; ok {
				newEventModel.ID = value.StringValue()
			}
			if value, ok := optionMap["title"]; ok {
				newEventModel.Summary = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["description"]; ok {
				newEventModel.Description = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["start"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse start date: %w", err)
				}
				newEventModel.StartDateUnixUTC = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["end"]; ok {
				result, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return nil, fmt.Errorf("can't parse end date: %w", err)
				}
				newEventModel.EndDateUnixUTC = result.Time.UTC().Unix()
			}
			if value, ok := optionMap["location"]; ok {
				newEventModel.Location = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["url"]; ok {
				if _, err := url.ParseRequestURI(value.StringValue()); err != nil {
					return nil, fmt.Errorf("invalid URL: %w", err)
				}
				newEventModel.URL = utils.CleanupString(value.StringValue())
			}
			if value, ok := optionMap["invitees"]; ok {
				rawString := value.StringValue()
				for _, attendee := range strings.Split(rawString, ",") {
					attendee := strings.TrimSpace(attendee)
					if attendee != "" {
						attendeeModels = append(attendeeModels, model.Attendee{
							EventID: newEventModel.ID,
							Data:    attendee,
						})
					}
				}
			}
			if value, ok := optionMap["whole-day"]; ok {
				newEventModel.IsWholeDay = value.BoolValue()
				startDate := time.Unix(newEventModel.StartDateUnixUTC, 0)
				endDate := time.Unix(newEventModel.EndDateUnixUTC, 0)
				if newEventModel.IsWholeDay {
					startDate = startDate.Truncate(24 * time.Hour)
					endDate = endDate.Truncate(24 * time.Hour)
				}
				newEventModel.StartDateUnixUTC = startDate.UTC().Unix()
				newEventModel.EndDateUnixUTC = endDate.UTC().Unix()
			}

			return newEventModel, nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Invalid event data\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("modifyEventHandler: can't respond about can't parse input data", "error", err)
			}
			return nil
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

			oldEvent := new(model.Event)
			if err := as.BunDB.
				NewSelect().
				Model(oldEvent).
				Where("id = ?", newEventModel.ID).
				Scan(context.Background()); err != nil {
				return false, false, fmt.Errorf("can't get old event: %w", err)
			}

			// these variables are only for the embed msg
			var title string
			description := "`None` `[unchanged]`"
			startDate := "`None` `[unchanged]`"
			endDate := "`None` `[unchanged]`"
			url := "`None` `[unchanged]`"
			location := "`None` `[unchanged]`"
			attendees := "`None` `[unchanged]`"

			switch newExist, oldExist := newEventModel.Summary != "", oldEvent.Summary != ""; {
			case newExist && oldExist:
				title = fmt.Sprintf("%s `[old value: %s]`", newEventModel.Summary, oldEvent.Summary)
			case newExist && !oldExist:
				title = fmt.Sprintf("%s `[old value: None]`", newEventModel.Summary)
			case !newExist && oldExist:
				title = fmt.Sprintf("%s `[unchanged]`", oldEvent.Summary)
			}

			switch newExist, oldExist := newEventModel.Description != "", oldEvent.Description != ""; {
			case newExist && oldExist:
				description = fmt.Sprintf("%s `[old value: %s]`", newEventModel.Description, oldEvent.Description)
			case newExist && !oldExist:
				description = fmt.Sprintf("%s `[old value: None]`", newEventModel.Description)
			case !newExist && oldExist:
				description = fmt.Sprintf("%s `[unchanged]`", oldEvent.Description)
			}

			switch newExist, oldExist := newEventModel.StartDateUnixUTC != 0, oldEvent.StartDateUnixUTC != 0; {
			case newExist && oldExist:
				startDate = fmt.Sprintf("<t:%d:f> `[old value: <t:%d:f>]`", newEventModel.StartDateUnixUTC, oldEvent.StartDateUnixUTC)
			case newExist && !oldExist:
				startDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEventModel.StartDateUnixUTC)
			case !newExist && oldExist:
				startDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.StartDateUnixUTC)
			}

			switch newExist, oldExist := newEventModel.EndDateUnixUTC != 0, oldEvent.EndDateUnixUTC != 0; {
			case newExist && oldExist:
				endDate = fmt.Sprintf("<t:%d:f> `[old value: <t:%d:f>]`", newEventModel.EndDateUnixUTC, oldEvent.EndDateUnixUTC)
			case newExist && !oldExist:
				endDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", newEventModel.EndDateUnixUTC)
			case !newExist && oldExist:
				endDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", oldEvent.EndDateUnixUTC)
			}

			switch newExist, oldExist := newEventModel.URL != "", oldEvent.URL != ""; {
			case newExist && oldExist:
				url = fmt.Sprintf("%s `[old value: %s]`", newEventModel.URL, oldEvent.URL)
			case newExist && !oldExist:
				url = fmt.Sprintf("%s `[old value: None]`", newEventModel.URL)
			case !newExist && oldExist:
				url = fmt.Sprintf("%s `[unchanged]`", oldEvent.URL)
			}

			switch newExist, oldExist := newEventModel.Location != "", oldEvent.Location != ""; {
			case newExist && oldExist:
				location = fmt.Sprintf("%s `[old value: %s]`", newEventModel.Location, oldEvent.Location)
			case newExist && !oldExist:
				location = fmt.Sprintf("%s `[old value: None]`", newEventModel.Location)
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
			newAttendees := func() string {
				var attendees []string
				for _, attendeeModel := range attendeeModels {
					attendees = append(attendees, attendeeModel.Data)
				}
				return strings.Join(attendees, ", ")
			}()
			switch newExist, oldExist := newAttendees != "", oldAttendees != ""; {
			case newExist && oldExist:
				attendees = fmt.Sprintf("%s `[old value: %s]`", newAttendees, oldAttendees)
			case newExist && !oldExist:
				attendees = fmt.Sprintf("%s `[old value: None]`", newAttendees)
			case !newExist && oldExist:
				attendees = fmt.Sprintf("%s `[unchanged]`", oldAttendees)
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
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't update event\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Error("modifyEventHandler: can't respond about can't ask for confirmation", "error", err)
			}
			return nil
		case timeout:
			// respond to nothing
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("modifyEventHandler: can't send timed out waiting for confirmation message", "error", err)
			}
			return nil
		case !isContinue:
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event not modified.",
				},
			}); err != nil {
				slog.Warn("modifyEventHandler: can't respond about event modification canceled", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - update event
		startTimer = time.Now()
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
			if len(attendeeModels) > 0 {
				if _, err := tx.NewInsert().
					Model(&attendeeModels).
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
				slog.Warn("modifyEventHandler: can't respond about can't update event in database", "error", err)
			}
			return fmt.Errorf("modifyEventHandler: can't update event in database: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// respond to button request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event updated.",
			},
		}); err != nil {
			slog.Warn("modifyEventHandler: can't respond about event update success", "error", err)
		}

		return nil
	}
}
