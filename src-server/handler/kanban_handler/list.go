package kanban_handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func list(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "list"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "List kanban groups.",
	})
	cmdHandler[id] = listHandler(as)
}

func listHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "list", "content", "deferring", "error", err)
		}

		// get all groups
		groups := make([]model.KanbanGroup, 0)
		if err := as.BunDB.
			NewSelect().
			Model(&groups).
			Where("channel_id = ?", interaction.ChannelID).
			Relation("Items").
			Scan(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get groups\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "list", "content", msg, "error", err)
			}
			return err
		}

		// compose & send the message
		embeds := []*discordgo.MessageEmbed{}
		for _, group := range groups {
			embeds = append(embeds, &discordgo.MessageEmbed{
				Title: group.Name,
				Description: func() string {
					var sb strings.Builder
					for _, item := range group.Items {
						sb.WriteString(fmt.Sprintf("`[%d]` %s\n", item.ID, item.Content))
					}
					return sb.String()
				}(),
			})
		}

		// edit the deferred message
		content := ""
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
			Embeds:  &embeds,
		}); err != nil {
			slog.Warn("can't respond", "handler", "list", "content", "list-groups-error", "error", err)
		}

		return nil
	}
}
