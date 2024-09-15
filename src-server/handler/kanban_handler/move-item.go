package kanban_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func moveItem(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "move-item"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Move an item to another group.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "group-name",
				Description: "The name of the group to move the item to.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "item-id",
				Description: "The ID of the item to move.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = moveItemHandler(as)
}

func moveItemHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to the original request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "move-item", "content", "deferring", "error", err)
		}

		// #region - get the content
		options := i.ApplicationCommandData().Options[0].Options
		optionMap := make(
			map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
		)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}
		var groupName string
		var itemID int64
		var err error
		if opt, ok := optionMap["group-name"]; ok {
			groupName = opt.StringValue()
		}
		if opt, ok := optionMap["item-id"]; ok {
			itemID, err = strconv.ParseInt(opt.StringValue(), 10, 64)
			if err != nil {
				// edit the deferred message
				msg := fmt.Sprintf("Can't parse item ID\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("can't respond", "handler", "move-item", "content", "can't-parse-item-id", "error", err)
				}
				return err
			}
		}

		if groupName == "" || itemID == 0 {
			// edit the deferred message
			msg := "Group name and item ID are required."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "group-name-item-id-required", "error", err)
			}
			return nil
		}

		// #region - check if group exists
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.KanbanGroup)(nil)).
			Where("name = ?", groupName).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't get group\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "can't-get-group", "error", err)
			}
			return err
		case !exists:
			// edit the deferred message
			msg := fmt.Sprintf("Group `%s` not found.", groupName)
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "group-not-found", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - check if item exists
		exists, err = as.BunDB.
			NewSelect().
			Model((*model.KanbanItem)(nil)).
			Where("id = ?", itemID).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't check if item exists\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "can't-check-if-item-exists", "error", err)
			}
			return err
		case !exists:
			// edit the deferred message
			msg := fmt.Sprintf("Item `%d` not found.", itemID)
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "item-not-found", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the item model
		itemModel := new(model.KanbanItem)
		if err := as.BunDB.
			NewSelect().
			Model(itemModel).
			Where("id = ?", itemID).
			Where("channel_id = ?", interaction.ChannelID).
			Scan(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get item\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "move-item", "content", "can't-get-item", "error", err)
			}
			return err
		}
		// #endregion

		// #region - move the item
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewDelete().
				Model((*model.KanbanItem)(nil)).
				Where("id = ?", itemID).
				Exec(ctx); err != nil {
				return err
			}
			_, err := tx.NewInsert().
				Model(model.KanbanItem{
					ID:        itemID,
					Content:   itemModel.Content,
					GroupName: groupName,
					ChannelID: interaction.ChannelID,
				}).
				Exec(ctx)
			return err
		}); err != nil {
			if err2 := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't create kanban group.\n```\n%s\n```", err.Error()),
				},
			}); err2 != nil {
				slog.Warn("can't respond", "handler", "create-kanban-group", "content", "create-kanban-group-error", "error", err)
			}
			return err
		}
		// #endregion

		// edit the deferred message
		content := fmt.Sprintf("Item `%d` moved to group `%s`.", itemID, groupName)
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		}); err != nil {
			slog.Warn("can't respond", "handler", "move-item", "content", "move-item-success", "error", err)
		}

		return nil
	}
}
