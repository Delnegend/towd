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
	"towd/src-server/ical/event"
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
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "import-calendar", "content", "deferring", "error", err)
		}

		// #region | parse url, name & validate url
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
			msg := "Invalid URL."
			msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
			}
			return fmt.Errorf(msgLower)
		}
		// #endregion

		// #region | check if calendar already exists in DB
		if exists, err := as.BunDB.
			NewSelect().
			Model((*model.Calendar)(nil)).
			Where("url = ?", url_).
			Exists(context.Background()); err != nil {
			return err
		} else if exists {
			msg := "Calendar already exists in the database."
			msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
			}
			return fmt.Errorf(msgLower)
		}
		// #endregion

		// #region | fetch & parse calendar
		var icalCalendar *ical.Calendar
		type icalParseResultChType struct {
			ok  *ical.Calendar
			err error
		}
		icalParseResultCh := make(chan icalParseResultChType)
		go func() {
			defer close(icalParseResultCh)
			iCalCalendar, err := ical.FromIcalUrl(url_)
			if err != nil {
				icalParseResultCh <- icalParseResultChType{err: err}
				return
			}
			icalParseResultCh <- icalParseResultChType{ok: iCalCalendar}
		}()
		select {
		case result := <-icalParseResultCh:
			if result.err != nil {
				return result.err
			}
			icalCalendar = result.ok
		case <-time.After(time.Minute * 5):
			msg := "Fetching and parsing the calendar took too long."
			msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
			}
			return fmt.Errorf(msgLower)
		}
		// #endregion

		// #region | handle if there's no calendar name
		if name_ != "" {
			icalCalendar.SetName(name_)
		} else if icalCalendar.GetName() == "" {
			// prepare IDs and channels
			addNameButtonID := "add-" + icalCalendar.GetID()
			cancelAddNameButtonID := "cancel-" + icalCalendar.GetID()
			addNameModalID := "modal-" + icalCalendar.GetID()
			calNameCh := make(chan string)
			errCh := make(chan error)
			defer close(calNameCh)
			defer close(errCh)

			as.AddAppCmdHandler(addNameButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
					slog.Warn("can't respond", "handler", "import-calendar", "content", "add name modal", "error", err)
				}
				return nil
			})
			as.AddAppCmdHandler(cancelAddNameButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				msg := "Calendar import canceled."
				msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
				if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: msg,
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
				}
				errCh <- fmt.Errorf(msgLower)
				return nil
			})
			as.AddAppCmdHandler(addNameModalID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				modalData := i.ModalSubmitData()
				calName := modalData.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
				calNameCh <- calName
				return nil
			})
			defer as.RemoveAppCmdHandler(addNameButtonID)
			defer as.RemoveAppCmdHandler(cancelAddNameButtonID)
			defer as.RemoveAppCmdHandler(addNameModalID)

			msg := "Seems like the calendar is missing a name."
			msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
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
				slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
				errCh <- fmt.Errorf(msgLower)
			}

			select {
			case calNameResult := <-calNameCh:
				icalCalendar.SetName(calNameResult)
			case err := <-errCh:
				return err
			case <-time.After(time.Minute * 5):
				msg := "Timed out waiting for entering calendar name."
				msgLower := strings.TrimSuffix(strings.ToLower(msg), ".")
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("can't respond", "handler", "import-calendar", "content", msgLower, "error", err)
				}
				return fmt.Errorf(msgLower)
			}
		}
		// #endregion

		// #region | confirm or cancel import
		confirmButtonID := "confirm-import-" + icalCalendar.GetID()
		cancelButtonID := "cancel-import-" + icalCalendar.GetID()

		confirmCh := make(chan struct{})
		cancelCh := make(chan struct{})
		defer close(confirmCh)
		defer close(cancelCh)
		msg := fmt.Sprintf("Found `%d` events in `%s`. Continue?", icalCalendar.GetMasterEventCount(), icalCalendar.GetName())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Components: &[]discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							CustomID: confirmButtonID,
							Label:    "Import",
							Style:    discordgo.PrimaryButton,
						}, discordgo.Button{
							CustomID: cancelButtonID,
							Label:    "Cancel",
							Style:    discordgo.SecondaryButton,
						},
					},
				},
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "import-calendar", "content", "importing", "error", err)
		}
		var buttonInteraction *discordgo.Interaction
		as.AddAppCmdHandler(confirmButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			buttonInteraction = i.Interaction
			confirmCh <- struct{}{}
			return nil
		})
		as.AddAppCmdHandler(cancelButtonID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			buttonInteraction = i.Interaction
			cancelCh <- struct{}{}
			return nil
		})
		defer as.RemoveAppCmdHandler(confirmButtonID)
		defer as.RemoveAppCmdHandler(cancelButtonID)

		// #region | run in transaction, import calendar
		var resultErr error
		select {
		case <-cancelCh:
			resultErr = fmt.Errorf("calendar import canceled")
		case <-confirmCh:
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				// create new calendar model and insert to DB
				calendarModel := model.Calendar{
					ID:          icalCalendar.GetID(),
					ProdID:      icalCalendar.GetProdID(),
					Name:        icalCalendar.GetName(),
					Description: icalCalendar.GetDescription(),
					Url:         url_,
				}
				if _, err := tx.
					NewInsert().
					Model(&calendarModel).
					Exec(ctx); err != nil {
					return err
				}

				// upsert all master events
				if err := icalCalendar.IterateMasterEvents(func(id string, icalMasterEvent event.MasterEvent) error {
					masterEventModel := new(model.MasterEvent)
					if err := masterEventModel.FromIcal(
						ctx, tx, &icalMasterEvent, calendarModel.ID); err != nil {
						return err
					}
					return masterEventModel.Upsert(ctx, tx)
				}); err != nil {
					return err
				}

				// upsert all child events
				if err := icalCalendar.IterateChildEvents(func(id string, icalChildEvent event.ChildEvent) error {
					childEventModel := new(model.ChildEvent)
					if err := childEventModel.FromIcal(
						ctx, tx, &icalChildEvent); err != nil {
						return err
					}
					return childEventModel.Upsert(ctx, tx)
				}); err != nil {
					return err
				}

				return nil
			}); err != nil {
				resultErr = fmt.Errorf("importCalendarHandler: %w", err)
			}
		case <-time.After(time.Minute * 2):
			s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation.")
			return nil
		}
		// #endregion

		if err := s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: func() string {
					if resultErr != nil {
						return msg
					}
					return fmt.Sprintf("%s\n\n%s", msg, "Imported events will be added to the calendar.")
				}(),
			},
		}); err != nil {
			slog.Error("can't respond", "handler", "import-calendar", "content", "import-calendar-error", "error", err)
		}

		if resultErr != nil {
			return fmt.Errorf("importCalendarHandler: %w", resultErr)
		}
		return nil
	}
}
