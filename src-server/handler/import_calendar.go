package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
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

		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("importCalendarHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// #region - parse calendarURL, nameOverride & validate calendarURL
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
		icalCalendar, err := func() (*ical.Calendar, error) {
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
				return nil, fmt.Errorf("timed out waiting for calendar to be fetched & parsed")
			case err := <-errCh:
				return nil, fmt.Errorf("can't fetch calendar: %w", err)
			case icalCal := <-calCh:
				return icalCal, nil
			}
		}()
		if err != nil {
			msg := fmt.Sprintf("Can't fetch calendar.\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about can't fetch calendar", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't fetch calendar: %w", err)
		}
		// #endregion

		// #region - prompt to enter calendar name if not provided & calendar not has one
		isCanceled, isTimedOut, err := func() (bool, bool, error) {
			switch {
			case nameOverride != "":
				icalCalendar.SetName(nameOverride)
				return false, false, nil
			case icalCalendar.GetName() != "":
				return false, false, nil
			}

			addNameButtonID := "add-" + icalCalendar.GetID()
			cancelAddNameButtonID := "cancel-" + icalCalendar.GetID()
			addNameModalID := "modal-" + icalCalendar.GetID()

			msg := "Seems like the calendar is missing a name."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Components: &[]discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								CustomID: addNameButtonID,
								Label:    "Give it one",
								Style:    discordgo.PrimaryButton,
							}, discordgo.Button{
								CustomID: cancelAddNameButtonID,
								Label:    "Cancel import",
								Style:    discordgo.SecondaryButton,
							},
						},
					}},
			}); err != nil {
				return false, false, fmt.Errorf("can't send msg to collect calendar name: %w", err)
			}

			// prepare channels
			calNameCh := make(chan string)
			cancelCh := make(chan struct{})
			errCh := make(chan error)
			defer close(calNameCh)
			defer close(cancelCh)
			defer close(errCh)

			// create buttons/modal handlers
			as.AddAppCmdHandler(addNameButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseModal,
					Data: &discordgo.InteractionResponseData{
						CustomID: addNameModalID,
						Title:    "Enter a name for the calendar",
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.TextInput{
										Label:     "Calendar name (max 300 characters)",
										Value:     "Lorem ipsum",
										Required:  true,
										Style:     discordgo.TextInputShort,
										MaxLength: 300,
									},
								},
							},
						},
					},
				}); err != nil {
					errCh <- fmt.Errorf("can't send modal to collect calendar name: %w", err)
				}
				return nil
			})
			as.AddAppCmdHandler(cancelAddNameButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				cancelCh <- struct{}{}
				return nil
			})
			as.AddAppCmdHandler(addNameModalID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				modalData := i.ModalSubmitData()
				calName := modalData.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
				calNameCh <- calName
				return nil
			})
			defer as.RemoveAppCmdHandler(addNameButtonID)
			defer as.RemoveAppCmdHandler(cancelAddNameButtonID)
			defer as.RemoveAppCmdHandler(addNameModalID)

			// wait for response
			select {
			case <-time.After(time.Minute * 2):
				return false, true, nil
			case <-cancelCh:
				return true, false, nil
			case err := <-errCh:
				return false, false, err
			case name := <-calNameCh:
				icalCalendar.SetName(name)
				return false, false, nil
			}
		}()
		switch {
		case err != nil:
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Calendar import canceled.\n```\n%s\n```", err.Error()),
				},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't respond about can't import calendar", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't import calendar: %w", err)
		case isCanceled:
			msg := "Calendar import canceled."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("importCalendarHandler: can't send message about calendar import canceled", "error", err)
			}
			return nil
		case isTimedOut:
			if _, err := s.ChannelMessageSend(interaction.ChannelID, "Calendar import canceled."); err != nil {
				slog.Warn("importCalendarHandler: can't send message about calendar import timed out", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - ask for confirmation to continue
		isCanceled, isTimedOut, err = func() (bool, bool, error) {
			cancelButtonID := "cancel-import-" + icalCalendar.GetID()
			confirmButtonID := "confirm-import-" + icalCalendar.GetID()
			cancelCh := make(chan struct{})
			confirmCh := make(chan struct{})
			defer close(cancelCh)
			defer close(confirmCh)

			// create buttons/modal handlers
			as.AddAppCmdHandler(cancelButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				cancelCh <- struct{}{}
				return nil
			})
			as.AddAppCmdHandler(confirmButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				confirmCh <- struct{}{}
				return nil
			})
			defer as.RemoveAppCmdHandler(confirmButtonID)
			defer as.RemoveAppCmdHandler(cancelButtonID)

			// send msg to ask for confirmation
			msg := fmt.Sprintf(
				"Found `%d` events in `%s`. Continue?",
				icalCalendar.GetMasterEventCount(),
				icalCalendar.GetName(),
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
				return false, false, fmt.Errorf("can't ask for confirmation: %w", err)
			}

			// waiting for response
			select {
			case <-time.After(time.Minute * 2):
				return false, true, nil
			case <-cancelCh:
				return true, false, nil
			case <-confirmCh:
				return false, false, nil
			}
		}()

		switch {
		case err != nil:
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Calendar import canceled.\n```\n%s\n```", err.Error()),
				},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't respond about can't ask for confirmation", "error", err)
			}
			return fmt.Errorf("importCalendarHandler: can't ask for confirmation: %w", err)
		case isCanceled:
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Calendar import canceled.",
				},
			}); err != nil {
				slog.Warn("importCalendarHandler: can't respond about calendar import canceled", "error", err)
			}
			return nil
		case isTimedOut:
			if _, err := s.ChannelMessageSend(interaction.ChannelID, "Calendar import canceled."); err != nil {
				slog.Warn("importCalendarHandler: can't send message about calendar import timed out", "error", err)
			}
			return nil
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
				ID:          icalCalendar.GetID(),
				ProdID:      icalCalendar.GetProdID(),
				Name:        icalCalendar.GetName(),
				Description: icalCalendar.GetDescription(),
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
			for _, staticEvent := range icalCalendar.ToStaticEvents() {
				eventModels = append(eventModels, model.Event{
					ID:               staticEvent.ID,
					Summary:          staticEvent.Title,
					Description:      staticEvent.Description,
					Location:         staticEvent.Location,
					URL:              staticEvent.URL,
					Organizer:        staticEvent.Organizer,
					StartDateUnixUTC: staticEvent.StartDate,
					EndDateUnixUTC:   staticEvent.EndDate,
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

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Calendar `%s` imported successfully.", icalCalendar.GetName()),
			},
		}); err != nil {
			slog.Warn("importCalendarHandler: can't respond about calendar import success", "error", err)
		}

		return nil
	}
}
