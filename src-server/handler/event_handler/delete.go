package event_handler

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func Delete_(as *utils.AppState) {
	id := "event-delete"
	as.AddAppCmdHandler(id, deleteHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Delete an event",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "ID of the event to delete.",
				Required:    true,
			},
		},
	})
}

func delete(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "delete"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Delete an event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "ID of the event to delete.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = deleteHandler(as)
}

func deleteHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to original request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "delete-event", "content", "deferring", "error", err)
		}

		eventID, err := func() (string, error) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var eventID string
			if opt, ok := optionMap["event-id"]; ok {
				eventID = opt.StringValue()
			}
			return eventID, nil
		}()
		if eventID == "" {
			// edit the deferred message
			msg := "Event ID is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-id-empty", "error", err)
			}
			return nil
		}

		eventModel, exist, err := func() (interface{}, bool, error) {
			switch regexp.MustCompile(`^[0-9]+$`).MatchString(eventID) {
			case true: // child event
				fmt.Println("child event")
				exist, err := as.BunDB.
					NewSelect().
					Model((*model.ChildEvent)(nil)).
					Where("recurrence_id = ?", eventID).
					Exists(context.Background())
				if err != nil {
					return nil, false, err
				}
				if !exist {
					return nil, false, nil
				}
				eventModel := new(model.ChildEvent)
				if err := as.BunDB.
					NewSelect().
					Model(eventModel).
					Where("recurrence_id = ?", eventID).
					Scan(context.Background()); err != nil {
					return nil, false, fmt.Errorf("can't get child event: %w", err)
				}
				return eventModel, true, nil
			default: // master event
				fmt.Println("master event", eventID)
				exist, err := as.BunDB.
					NewSelect().
					Model(&model.MasterEvent{}).
					Where("id = ?", eventID).
					Exists(context.Background())
				if err != nil {
					return nil, false, err
				}
				if !exist {
					return nil, false, nil
				}
				eventModel := new(model.MasterEvent)
				if err := as.BunDB.
					NewSelect().
					Model(eventModel).
					Where("id = ?", eventID).
					Scan(context.Background()); err != nil {
					return nil, false, fmt.Errorf("can't get master event: %w", err)
				}
				return eventModel, true, nil
			}
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't check if event exists\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
			}
			return err
		} else if !exist {
			// edit the deferred message
			msg := "Event not found."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
			}
			return nil
		}

		if isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventID
			cancelCustomId := "cancel-" + eventID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			var embeds []*discordgo.MessageEmbed
			switch eventModel := eventModel.(type) {
			case *model.MasterEvent:
				embeds = eventModel.ToDiscordEmbed(context.Background(), as.BunDB)
			case *model.ChildEvent:
				embeds = eventModel.ToDiscordEmbed(context.Background(), as.BunDB)
			}

			// edit the deferred message
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: func() *string {
					if len(embeds) == 1 {
						msg := "Is this the event you want to delete?"
						return &msg
					}
					msg := "Are these the events you want to delete?"
					return &msg
				}(),
				Embeds: &embeds,
				Components: &[]discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Yes",
								Style:    discordgo.SuccessButton,
								CustomID: yesCustomId,
							},
							discordgo.Button{
								Label:    "No",
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
		}(); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't ask for confirmation, can't continue: %s", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "ask-for-confirmation", "error", err)
			}
			return err
		} else if timeout {
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "timeout", "error", err)
			}
			return nil
		} else if !isContinue {
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event deletion canceled.",
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "cancel", "error", err)
			}
			return nil
		}

		msg := "Event deleted."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-success", "error", err)
		}

		return nil
	}
}
