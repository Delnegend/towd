package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func ModifyEvent(as *utils.AppState) {
	id := "modify-event"
	as.AddAppCmdHandler(id, modifyEventHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Modify an existing calendar event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "The ID of the event to modify",
				Required:    true,
			},
		},
	})
}

func modifyEventHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		// #region | param -> event ID
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(i.ApplicationCommandData().Options))
		for _, opt := range i.ApplicationCommandData().Options {
			optionMap[opt.Name] = opt
		}
		var eventID string
		if opt, ok := optionMap["event-id"]; ok {
			eventID = opt.StringValue()
		}
		if eventID == "" {
			msg := "Event ID is required."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-id-required", "error", err)
				return fmt.Errorf("modifyEventHandler: %w", err)
			}
		}
		// #endregion

		// #region | check if is modifying a master event or a child event
		isMasterEvent := false
		switch regexp.MustCompile(`^[0-9]+$`).MatchString(eventID) {
		case false: // master event
			exist, err := as.BunDB.
				NewSelect().
				Model((*model.MasterEvent)(nil)).
				Where("id = ?", eventID).
				Exists(context.Background())
			if err != nil || !exist {
				msg := fmt.Sprintf("Can't find event with ID `%s`\n```%s```", eventID, err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "event-not-found", "error", err)
					return fmt.Errorf("modifyEventHandler: %w", err)
				}
			}
			isMasterEvent = true
		case true: // child event
			exist, err := as.BunDB.
				NewSelect().
				Model((*model.ChildEvent)(nil)).
				Where("event_id = ?", eventID).
				Exists(context.Background())
			if err != nil || !exist {
				msg := fmt.Sprintf("Can't find event with ID `%s`\n```%s```", eventID, err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "event-not-found", "error", err)
					return fmt.Errorf("modifyEventHandler: %w", err)
				}
			}
		}
		// #endregion

		// #region | send a model and collect data
		customID := "modify-" + eventID
		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "title",
						Label:    "Title",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "description",
						Label:    "Description",
						Style:    discordgo.TextInputParagraph,
						Required: false,
						Value:    "leave blank for no change",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "start",
						Label:    "Start Time",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change | format: YYYY/MM/DD HH:mm:ss",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "end",
						Label:    "End Time",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change | format: YYYY/MM/DD HH:mm:ss",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "location",
						Label:    "Location",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change",
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "url",
						Label:    "URL",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change",
					},
				},
			},
		}
		if isMasterEvent {
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "rrule",
						Label:    "RRule",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change",
					},
				},
			})
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "rdate",
						Label:    "RDate",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change | each separated by comma",
					},
				},
			})
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "exdate",
						Label:    "ExDate",
						Style:    discordgo.TextInputShort,
						Required: false,
						Value:    "leave blank for no change | each separated by comma",
					},
				},
			})
		}
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID:   customID,
				Title:      "Modify Event",
				Components: components,
			},
		}); err != nil {
			slog.Error("can't respond", "handler", "modify-event", "content", "event-modify-modal", "error", err)
			return fmt.Errorf("modifyEventHandler: %w", err)
		}

		// collect data
		dataMap := make(map[string]string, 0)
		var wg sync.WaitGroup
		wg.Add(1)
		as.AddAppCmdHandler(customID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			defer wg.Done()
			for _, opt := range i.ModalSubmitData().Components {
				actionRow, ok := opt.(*discordgo.ActionsRow)
				if !ok {
					continue
				}
				if len(actionRow.Components) != 1 {
					continue
				}
				textInput, ok := actionRow.Components[0].(*discordgo.TextInput)
				if !ok {
					continue
				}
				value := textInput.Value
				dataMap[textInput.CustomID] = value
			}
			return nil
		})
		defer as.RemoveAppCmdHandler(customID)
		wg.Wait()
		// #endregions

		// #region | preprocess data
		if exdate, ok := dataMap["exdate"]; ok {
			exdateSlice := strings.Split(exdate, ",")
			exdateUnix := make([]string, len(exdateSlice))
			for _, date := range exdateSlice {
				date = strings.TrimSpace(date)
				if result, err := as.When.Parse(date, time.Now()); err != nil {
					msg := fmt.Sprintf("Can't parse exdate\n```\n%s\n```", err.Error())
					if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content: &msg,
					}); err != nil {
						slog.Error("can't respond", "handler", "modify-event", "content", "parse-date-error", "error", err)
					}
					return nil
				} else {
					exdateUnix = append(exdateUnix, string(result.Time.UTC().Unix()))
				}
			}
			dataMap["exdate"] = strings.Join(exdateUnix, ",")
		}
		if rdate, ok := dataMap["rdate"]; ok {
			rdateSlice := strings.Split(rdate, ",")
			rdateUnix := make([]string, len(rdateSlice))
			for _, date := range rdateSlice {
				date = strings.TrimSpace(date)
				if result, err := as.When.Parse(date, time.Now()); err != nil {
					msg := fmt.Sprintf("Can't parse rdate\n```\n%s\n```", err.Error())
					if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content: &msg,
					}); err != nil {
						slog.Error("can't respond", "handler", "modify-event", "content", "parse-date-error", "error", err)
					}
					return nil
				} else {
					rdateUnix = append(rdateUnix, string(result.Time.UTC().Unix()))
				}
			}
			dataMap["rdate"] = strings.Join(rdateUnix, ",")
		}
		if start, ok := dataMap["start"]; ok {
			if result, err := as.When.Parse(start, time.Now()); err != nil {
				msg := fmt.Sprintf("Can't parse start date\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "parse-date-error", "error", err)
				}
				return nil
			} else {
				dataMap["start"] = string(result.Time.UTC().Unix())
			}
		}
		if end, ok := dataMap["end"]; ok {
			if result, err := as.When.Parse(end, time.Now()); err != nil {
				msg := fmt.Sprintf("Can't parse end date\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "parse-date-error", "error", err)
				}
				return nil
			} else {
				dataMap["end"] = string(result.Time.UTC().Unix())
			}
		}
		// #endregion

		switch isMasterEvent {
		case true:
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				eModel := new(model.MasterEvent)
				if err := tx.NewSelect().Model(&eModel).Where("id = ?", eventID).Scan(ctx); err != nil {
					return fmt.Errorf("modifyEventHandler: %w", err)
				}
				for k, v := range dataMap {
					switch k {
					case "title":
						eModel.Summary = v
					case "description":
						eModel.Description = v
					case "start":
						v, _ := strconv.ParseInt(v, 10, 64)
						eModel.StartDate = int64(v)
					case "end":
						v, _ := strconv.ParseInt(v, 10, 64)
						eModel.EndDate = int64(v)
					case "location":
						eModel.Location = v
					case "url":
						eModel.URL = v
					case "rrule":
						eModel.RRule = v
					case "rdate":
						eModel.RDate = v
					case "exdate":
						eModel.ExDate = v
					}
				}
				return eModel.Upsert(ctx, tx)
			}); err != nil {
				msg := fmt.Sprintf("Can't update event\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "event-update-error", "error", err)
				}
				return fmt.Errorf("modifyEventHandler: %w", err)
			}
			msg := "Event updated."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-update-success", "error", err)
			}
		case false:
			if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				eModel := new(model.ChildEvent)
				if err := tx.NewSelect().Model(&eModel).Where("id = ?", eventID).Scan(ctx); err != nil {
					return fmt.Errorf("modifyEventHandler: %w", err)
				}
				for k, v := range dataMap {
					switch k {
					case "title":
						eModel.Summary = v
					case "description":
						eModel.Description = v
					case "start":
						v, _ := strconv.ParseInt(v, 10, 64)
						eModel.StartDate = int64(v)
					case "end":
						v, _ := strconv.ParseInt(v, 10, 64)
						eModel.EndDate = int64(v)
					case "location":
						eModel.Location = v
					case "url":
						eModel.URL = v
					}
				}
				return eModel.Upsert(ctx, tx)
			}); err != nil {
				msg := fmt.Sprintf("Can't update event\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "modify-event", "content", "event-update-error", "error", err)
				}
				return fmt.Errorf("modifyEventHandler: %w", err)
			}
			msg := "Event updated."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-update-success", "error", err)
			}
		}

		return nil
	}
}
