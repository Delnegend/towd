package event_handler

import (
	"context"
	"fmt"
	"log/slog"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

// Init injects one "event" slash command with multiple subcommands
// into appCmdInfo and appCmdHandler in AppState.
func Init(as *utils.AppState) {
	// works similar to how we create a new slash command using
	// appCmdInfo and appCmdHandler in AppState.
	localCmdInfo := make(
		[]*discordgo.ApplicationCommandOption, 0,
	)
	localCmdHandler := make(
		map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error,
	)

	// injecting info and handler into 2 local maps
	create(as, &localCmdInfo, localCmdHandler)
	natural(as, &localCmdInfo, localCmdHandler)
	delete(as, &localCmdInfo, localCmdHandler)
	list(as, &localCmdInfo, localCmdHandler)
	modify(as, &localCmdInfo, localCmdHandler)

	id := "event"
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Event management commands.",
		Options:     localCmdInfo,
	})
	as.AddAppCmdHandler(id, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		data := i.ApplicationCommandData()
		if handler, ok := localCmdHandler[data.Options[0].Name]; ok {
			return handler(s, i)
		}
		return nil
	})
}

func ensureCalendarExists(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	exists, err := as.BunDB.
		NewSelect().
		Model((*model.Calendar)(nil)).
		Where("channel_id = ?", i.ChannelID).
		Exists(context.Background())
	switch {
	case err != nil:
		// edit the deferred message
		msg := fmt.Sprintf("Can't check if calendar exists\n```\n%s\n```", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("ensureCalendarExists: can't send message about can't check if calendar exists", "error", err)
		}
		return fmt.Errorf("ensureCalendarExists: can't check if calendar exists: %w", err)
	case !exists:
		channels, err := as.DgSession.GuildChannels(i.GuildID)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get channel name to create calendar\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("ensureCalendarExists: can't send message about can't get channel name to create calendar", "error", err)
			}
			return fmt.Errorf("ensureCalendarExists: can't get channel name to create calendar: %w", err)
		}
		var channelName string
		for _, channel := range channels {
			if channel.ID == i.ChannelID {
				channelName = channel.Name
				break
			}
		}
		if channelName == "" {
			channelName = "Untitled"
		}

		if _, err := as.BunDB.NewInsert().
			Model(&model.Calendar{
				ChannelID: i.ChannelID,
				Name:      channelName,
			}).
			Exec(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't insert calendar to database\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("ensureCalendarExists: can't send message about can't insert calendar to database", "error", err)
			}
			return fmt.Errorf("ensureCalendarExists: can't insert calendar to database: %w", err)
		}
	}
	return nil
}
