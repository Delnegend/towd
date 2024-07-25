package handler

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func DeleteEvent(as *utils.AppState) {
	id := "delete-event"
	as.AddAppCmdHandler(id, deleteEventHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Delete an event",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-id",
				Description: "The event ID",
				Required:    true,
			},
		},
	})
}

func deleteEventHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
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
				return fmt.Errorf("deleteEventHandler: %w", err)
			}
		}
		// #endregion

		switch regexp.MustCompile(`^[0-9]+$`).MatchString(eventID) {
		case false: // master event
			if _, err := as.BunDB.
				NewDelete().
				Model((*model.MasterEvent)(nil)).
				Where("id = ?", eventID).
				Exec(context.Background()); err != nil {
				msg := fmt.Sprintf("Can't delete event\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
				}
				return fmt.Errorf("deleteEventHandler: %w", err)
			}
		case true: // child event
			if _, err := as.BunDB.
				NewDelete().
				Model((*model.ChildEvent)(nil)).
				Where("recurrence_id = ?", eventID).
				Exec(context.Background()); err != nil {
				msg := fmt.Sprintf("Can't delete event\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Error("can't respond", "handler", "delete-event", "content", "event-delete-error", "error", err)
				}
				return fmt.Errorf("deleteEventHandler: %w", err)
			}
		}

		msg := "Event deleted."
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Error("can't respond", "handler", "delete-event", "content", "event-delete-success", "error", err)
		}

		return nil
	}
}
