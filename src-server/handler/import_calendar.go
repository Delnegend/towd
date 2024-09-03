package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"time"
	"towd/src-server/ical"
	"towd/src-server/ical/event"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
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

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "import-calendar", "content", "deferring", "error", err)
		}

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
				slog.Warn("can't respond", "handler", "import-calendar", "content", "invalid-url", "error", err)
			}
			return err
		}
		// #endregion

		// #region - calendar already exists?
		if exists, err := as.BunDB.
			NewSelect().
			Model((*model.Calendar)(nil)).
			Where("url = ?", calendarURL).
			Exists(context.Background()); err != nil {
			return err
		} else if exists {
			msg := "Calendar already exists in the database."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "calendar-already-exists", "error", err)
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
				slog.Warn("can't respond", "handler", "import-calendar", "content", "fetch-calendar-error", "error", err)
			}
			return err
		}
		// #endregion

		// #region - prompt to enter calendar name if not provided & calendar not has one
		if isCanceled, isTimedOut, err := func() (bool, bool, error) {
			if nameOverride != "" {
				icalCalendar.SetName(nameOverride)
				return false, false, nil
			}
			if icalCalendar.GetName() != "" {
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
		}(); err != nil {
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Calendar import canceled.\n```\n%s\n```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-error", "error", err)
			}
			return err
		} else if isCanceled {
			msg := "Calendar import canceled."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-cancel", "error", err)
			}
			return nil
		} else if isTimedOut {
			if _, err := s.ChannelMessageSend(interaction.ChannelID, "Calendar import canceled."); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-timeout", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - ask for confirmation to continue
		if isCanceled, isTimedOut, err := func() (bool, bool, error) {
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
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-error", "error", err)
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
		}(); err != nil {
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Calendar import canceled.\n```\n%s\n```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-error", "error", err)
			}
			return err
		} else if isCanceled {
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Calendar import canceled.",
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-cancel", "error", err)
			}
			return nil
		} else if isTimedOut {
			if _, err := s.ChannelMessageSend(interaction.ChannelID, "Calendar import canceled."); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", "add-calendar-name-timeout", "error", err)
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
			calendarModel := model.Calendar{
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

			// extract master events and child events
			masterEventModels := make([]model.MasterEvent, 0)
			childEventModels := make([]model.ChildEvent, 0)
			if err := icalCalendar.IterateMasterEvents(func(id string, icalMasterEvent *event.MasterEvent) error {
				masterEventModels = append(masterEventModels, model.MasterEvent{
					ID:          id,
					Summary:     icalMasterEvent.GetSummary(),
					Description: icalMasterEvent.GetDescription(),
					Location:    icalMasterEvent.GetLocation(),
					URL:         icalMasterEvent.GetURL(),
					Organizer:   icalMasterEvent.GetOrganizer(),

					StartDate: icalMasterEvent.GetStartDate(),
					EndDate:   icalMasterEvent.GetEndDate(),

					CreatedAt: icalMasterEvent.GetCreatedAt(),
					UpdatedAt: icalMasterEvent.GetUpdatedAt(),
					Sequence:  icalMasterEvent.GetSequence(),

					CalendarID: calendarModel.ID,
					ChannelID:  interaction.ChannelID,
				})

				return icalMasterEvent.IterateChildEvents(func(id string, icalChildEvent *event.ChildEvent) error {
					childEventModels = append(childEventModels, model.ChildEvent{
						ID:            uuid.NewString(),
						MasterEventID: id,
						RecurrenceID:  icalChildEvent.GetRecurrenceID(),

						Summary:     icalChildEvent.GetSummary(),
						Description: icalChildEvent.GetDescription(),
						Location:    icalChildEvent.GetLocation(),
						URL:         icalChildEvent.GetURL(),
						Organizer:   icalChildEvent.GetOrganizer(),

						StartDate: icalChildEvent.GetStartDate(),
						EndDate:   icalChildEvent.GetEndDate(),

						CreatedAt: icalChildEvent.GetCreatedAt(),
						UpdatedAt: icalChildEvent.GetUpdatedAt(),
						Sequence:  icalChildEvent.GetSequence(),

						CalendarID: calendarModel.ID,
						ChannelID:  interaction.ChannelID,
					})
					return nil
				})
			}); err != nil {
				return err
			}

			// insert all events
			if _, err := tx.NewInsert().
				Model(&masterEventModels).
				Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewInsert().
				Model(&childEventModels).
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
				slog.Warn("can't respond", "handler", "import-calendar", "content", "import-calendar-error", "error", err)
			}
			return err
		}
		// #endregion

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Calendar `%s` imported successfully.", icalCalendar.GetName()),
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "import-calendar", "content", "import-calendar-success", "error", err)
		}

		return nil
	}
}
