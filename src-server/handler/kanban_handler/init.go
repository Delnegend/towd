package kanban_handler

import (
	"context"
	"fmt"
	"log/slog"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func Init(as *utils.AppState) {
	// works similar to how we create a new slash command using
	// appCmdInfo and appCmdHandler in AppState.
	localCmdInfo := make(
		[]*discordgo.ApplicationCommandOption, 0,
	)
	localCmdHandler := make(
		map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error,
	)

	list(as, &localCmdInfo, localCmdHandler)
	createItem(as, &localCmdInfo, localCmdHandler)
	createGroup(as, &localCmdInfo, localCmdHandler)
	moveItem(as, &localCmdInfo, localCmdHandler)
	deleteItem(as, &localCmdInfo, localCmdHandler)

	id := "kanban"
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Kanban management commands.",
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

func ensureKanbantableExists(as *utils.AppState, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	exists, err := as.BunDB.
		NewSelect().
		Model((*model.KanbanTable)(nil)).
		Where("channel_id = ?", i.ChannelID).
		Exists(context.Background())
	switch {
	case err != nil:
		// edit the deferred message
		msg := fmt.Sprintf("Can't check if kanban table exists\n```\n%s\n```", err.Error())
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("ensureKanbantableExists: can't send message about can't check if kanban table exists", "error", err)
		}
		return fmt.Errorf("ensureKanbantableExists: can't check if kanban table exists: %w", err)
	case !exists:
		channels, err := as.DgSession.GuildChannels(i.GuildID)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get channel name to create kanban board\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("ensureKanbantableExists: can't send message about can't get channel name to create kanban board", "error", err)
			}
			return fmt.Errorf("ensureKanbantableExists: can't get channel name to create kanban board: %w", err)
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
			Model(&model.KanbanTable{
				ChannelID: i.ChannelID,
				Name:      channelName,
			}).
			Exec(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create kanban board\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				return fmt.Errorf("ensureKanbantableExists: can't send message about can't insert kanban table to database: %w", err)
			}
			return fmt.Errorf("ensureKanbantableExists: can't insert kanban table to database: %w", err)
		}
	}
	return nil
}
