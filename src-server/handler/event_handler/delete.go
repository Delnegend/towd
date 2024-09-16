package event_handler

import (
	"context"
	"fmt"
	"log/slog"
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

		// #region - get the event ID
		eventID := func() string {
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
			return eventID
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
		// #endregion

		// #region - check if event exists
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.Event)(nil)).
			Where("id = ?", eventID).
			Exists(context.Background())
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't check if event exists\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
			}
			return err
		case !exists:
			// edit the deferred message
			msg := "Event not found."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the event model
		eventModel := new(model.Event)
		if err := as.BunDB.
			NewSelect().
			Model(eventModel).
			Where("id = ?", eventID).
			Scan(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get event\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
			}
			return err
		}
		// #endregion

		// #region - ask for confirmation
		isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventID
			cancelCustomId := "cancel-" + eventID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// edit the deferred message
			msg := "Is this the event you want to delete?"
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds: &[]*discordgo.MessageEmbed{
					eventModel.ToDiscordEmbed(context.Background(), as.BunDB),
				},
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
		}()
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't ask for confirmation, can't continue: %s", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "ask-for-confirmation", "error", err)
			}
			return err
		case timeout:
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("can't respond", "handler", "delete-event", "content", "timeout", "error", err)
			}
			return nil
		case !isContinue:
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

		// #endregion

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event deleted.",
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "delete-event", "content", "event-delete-success", "error", err)
		}

		return nil
	}
}
