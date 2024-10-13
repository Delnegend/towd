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

func handleActionTypeRead(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate, naturalOutput utils.NaturalOutput) error {
	// #region - get the events
	startDate, err := time.ParseInLocation("02/01/2006 15:04", naturalOutput.Body.StartDateToQuery, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't read event, invalid start date:\n```%s\n\n%s```\n", err.Error(), naturalOutput.Body.StartDateToQuery)
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:read: can't respond about can't read event, invalid start date", "error", err)
		}
		return nil
	}
	endDate, err := time.ParseInLocation("02/01/2006 15:04", naturalOutput.Body.EndDateToQuery, as.Config.GetLocation())
	if err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't read event, invalid end date:\n```%s\n\n%s```\n", err.Error(), naturalOutput.Body.EndDateToQuery)
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:read: can't respond about can't read event, invalid end date", "error", err)
		}
		return nil
	}
	eventModels := make([]model.Event, 0)
	if err := as.BunDB.
		NewSelect().
		Model(&eventModels).
		Where("channel_id = ?", i.ChannelID).
		Where("start_date >= ?", startDate.UTC().Unix()).
		Where("start_date <= ?", endDate.UTC().Unix()).
		Scan(context.Background()); err != nil {
		// edit the deferred message
		msg := fmt.Sprintf("Can't read event\n```\n%s\n```", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:read: can't respond about can't read event", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - compose response
	var embeds []*discordgo.MessageEmbed
	for _, eventModel := range eventModels {
		embeds = append(embeds, eventModel.ToDiscordEmbed())
	}
	if len(embeds) == 0 {
		// edit the deferred message
		msg := "No events found."
		if naturalOutput.Description != "" {
			msg = fmt.Sprintf("%s\n```\n%s\n```", naturalOutput.Description, msg)
		}
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("event_handler:natural:read: can't respond about no events found", "error", err)
		}
		return nil
	}
	// #endregion

	// #region - edit the deferred msg
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &naturalOutput.Description,
		Embeds:  &embeds,
	}); err != nil {
		slog.Warn("event_handler:natural:read: can't respond about can't read event", "error", err)
	}
	// #endregion

	return nil
}
