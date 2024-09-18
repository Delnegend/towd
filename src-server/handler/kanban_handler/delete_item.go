package kanban_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func deleteItem(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "delete-item"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Delete an item.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "item-id",
				Description: "The ID of the item to delete.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = deleteItemHandler(as)
}

func deleteItemHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("deleteItemHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// get the content
		options := i.ApplicationCommandData().Options[0].Options
		optionMap := make(
			map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
		)
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}
		var itemID int64
		var err error
		if opt, ok := optionMap["item-id"]; ok {
			itemID, err = strconv.ParseInt(opt.StringValue(), 10, 64)
			if err != nil {
				// edit the deferred message
				msg := fmt.Sprintf("Can't parse item ID\n```%s```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("deleteItemHandler: can't send message about can't parse item ID", "error", err)
				}
				return nil
			}
		}

		// delete the item
		startTimer = time.Now()
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewDelete().
				Model((*model.KanbanItem)(nil)).
				Where("id = ?", itemID).
				Exec(ctx); err != nil {
				msg := fmt.Sprintf("Can't delete kanban item\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("deleteItemHandler: can't send message about can't delete item", "error", err)
				}
				return fmt.Errorf("deleteItemHandler: can't delete item: %w", err)
			}
			return nil
		}); err != nil {
			if err2 := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't delete kanban item.\n```\n%s\n```", err.Error()),
				},
			}); err2 != nil {
				slog.Warn("deleteItemHandler: can't respond about can't delete item", "error", err)
			}
			return fmt.Errorf("deleteItemHandler: can't delete item: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())

		// edit the deferred message
		content := fmt.Sprintf("Item `%d` deleted.", itemID)
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		}); err != nil {
			slog.Warn("deleteItemHandler: can't respond about item deletion success", "error", err)
		}

		return nil
	}
}
