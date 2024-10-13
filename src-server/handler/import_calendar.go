package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"towd/src-server/ical"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func ImportCalendar(as *utils.AppState) {
	id := "import-calendar"
	as.AddAppCmdHandler(id, importCalendarHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Import an external calendar.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "The URL of the external calendar",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Override the calendar name",
				Required:    false,
			},
		},
	})
}

func importCalendarHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// #region - respond w/ deferred
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			slog.Warn("importCalendarHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// #region - parse input parameters & validate URL
		calendarURL, nameOverride, err := func() (string, string, error) {
			options := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, 0)
			for _, opt := range i.ApplicationCommandData().Options {
				options[opt.Name] = opt
			}
			var url_ string
			if opt, ok := options["url"]; ok {
				url_ = opt.StringValue()
			}
			var name_ string
			if opt, ok := options["name"]; ok {
				name_ = opt.StringValue()
			}
			if _, err := url.ParseRequestURI(url_); err != nil {
				return "", "", err
			}
			return url_, name_, nil
		}()
		if err != nil {
			msg := err.Error()
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about invalid URL", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - calendar already exists?
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.ExternalCalendar)(nil)).
			Where("url = ?", calendarURL).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			msg := fmt.Sprintf("Can't check if calendar exists\n```\n%s\n```", err.Error())
			if _, err2 := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err2 != nil {
				slog.Warn("importCalendarHandler: can't send message about can't check if calendar exists", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't check if calendar exists: %w", err)
		case exists:
			msg := "Calendar already exists in the database."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about calendar already exists", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - fetch & parse calendar
		calendar, isTimedOut, err := func() (*ical.Calendar, bool, error) {
			calCh := make(chan *ical.Calendar)
			errCh := make(chan error)
			defer close(calCh)
			defer close(errCh)
			go func() {
				iCalCalendar, err := ical.FromIcalUrl(calendarURL)
				if err != nil {
					errCh <- err
					return
				}
				calCh <- iCalCalendar
			}()
			select {
			case <-time.After(time.Minute * 5):
				return nil, true, nil
			case err := <-errCh:
				return nil, false, err
			case icalCal := <-calCh:
				return icalCal, false, nil
			}
		}()
		switch {
		case err != nil:
			msg := fmt.Sprintf("Can't fetch calendar.\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about can't fetch calendar", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't fetch calendar: %w", err)
		case isTimedOut:
			msg := "Timed out waiting for calendar to be fetched & parsed."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about timed out waiting for calendar to be fetched & parsed", "error", err)
			}
			return nil
		case calendar == nil:
			msg := "Something went wrong."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about something went wrong", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: something went wrong, iCalendar is not supposed to be nil")
		}
		// #endregion

		// #region - overwrite calendar name if provided
		if nameOverride != "" {
			calendar.SetName(nameOverride)
		} else if calendar.GetName() == "" {
			calendar.SetName("Untitled")
		}
		// #endregion

		// #region - ask for confirmation to continue
		cancelButtonID := "cancel-import-calendar" + calendar.GetID()
		cancelCh := make(chan struct{}, 1)
		defer close(cancelCh)
		as.AddAppCmdHandler(cancelButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			interaction = i.Interaction
			cancelCh <- struct{}{}
			return nil
		})
		defer as.RemoveAppCmdHandler(cancelButtonID)

		confirmButtonID := "confirm-import-calendar" + calendar.GetID()
		confirmCh := make(chan struct{}, 1)
		defer close(confirmCh)
		as.AddAppCmdHandler(confirmButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			interaction = i.Interaction
			confirmCh <- struct{}{}
			return nil
		})
		defer as.RemoveAppCmdHandler(confirmButtonID)

		// send msg to ask for confirmation
		msg := fmt.Sprintf(
			"Found `%d` events in `%s`. Continue?",
			calendar.GetMasterEventCount(),
			calendar.GetName(),
		)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Components: &[]discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							CustomID: confirmButtonID,
							Label:    "Import",
							Style:    discordgo.PrimaryButton,
						},
						discordgo.Button{
							CustomID: cancelButtonID,
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
						},
					},
				}},
		}); err != nil {
			return fmt.Errorf("importCalendarHandler: can't ask for confirmation: %w", err)
		}

		select {
		case <-cancelCh:
			// response the cancel button
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Calendar import canceled.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't respond about calendar import canceled", "error", err)
			}
			return nil
		case <-time.After(time.Minute * 2):
			// edit the ask-to-confirm message
			msg := "Timed out waiting for confirmation."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content:    &msg,
				Components: &[]discordgo.MessageComponent{},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't respond about calendar import timed out", "error", err)
			}
			return nil
		case <-confirmCh:
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send defer message to later edit to calendar import success", "error", err)
			}
		}
		// #endregion

		// #region - insert to DB
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			// create new calendar model and insert to DB
			hash, err := utils.GetFileHash(calendarURL)
			if err != nil {
				return err
			}
			calendarModel := model.ExternalCalendar{
				ID:          calendar.GetID(),
				ProdID:      calendar.GetProdID(),
				Name:        calendar.GetName(),
				Description: calendar.GetDescription(),
				Url:         calendarURL,
				Hash:        hash,
				ChannelID:   interaction.ChannelID,
			}
			if _, err := tx.
				NewInsert().
				Model(&calendarModel).
				Exec(ctx); err != nil {
				return err
			}

			eventModels := make([]model.Event, 0)
			for _, event := range calendar.ToStaticEvents() {
				eventModels = append(eventModels, model.Event{
					ID:               fmt.Sprintf("%s-%s", event.ID, i.ChannelID),
					Summary:          event.Title,
					Description:      event.Description,
					Location:         event.Location,
					URL:              event.URL,
					Organizer:        event.Organizer,
					StartDateUnixUTC: event.StartDate,
					EndDateUnixUTC:   event.EndDate,
					CalendarID:       calendarModel.ID,
					ChannelID:        interaction.ChannelID,
				})
			}
			if _, err := tx.NewInsert().
				Model(&eventModels).
				Exec(ctx); err != nil {
				return err
			}

			return nil
		}); err != nil {
			// response the confirm button
			if err2 := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't import calendar.\n```\n%s\n```", err.Error()),
				},
			}); err2 != nil {
				slog.Warn("importCalendarHandler: can't respond about can't import calendar", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't import calendar: %w", err)
		}
		// #endregion

		// #region - response the confirm button (ephemeral) & announce the calendar import
		msg = fmt.Sprintf("Calendar [%s](%s) imported successfully.", calendar.GetName(), calendarURL)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("importCalendarHandler: can't respond about calendar import success", "error", err)
		}

		msg = func() string {
			var sb strings.Builder
			if i.Member != nil && i.Member.User != nil {
				sb.WriteString(fmt.Sprintf("<@%s> imported", i.Member.User.ID))
			} else {
				sb.WriteString("Imported")
			}
			sb.WriteString(fmt.Sprintf(" calendar [%s](%s).", calendar.GetName(), calendarURL))
			return sb.String()
		}()
		if _, err := s.ChannelMessageSend(interaction.ChannelID, msg); err != nil {
			slog.Warn("importCalendarHandler: can't send message about calendar import success", "error", err)
		}
		// #endregion

		return nil
	}
}
