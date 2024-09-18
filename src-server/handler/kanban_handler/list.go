package kanban_handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
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
	cmdHandler[id] = listKanbanHandler(as)
}

func listKanbanHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("listKanbanHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// get all groups
		groups := make([]model.KanbanGroup, 0)
		startTimer = time.Now()
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
				slog.Warn("listKanbanHandler: can't send message about can't get groups", "error", err)
			}
			return fmt.Errorf("listKanbanHandler: can't get groups: %w", err)
		}
		as.MetricChans.DatabaseRead <- float64(time.Since(startTimer).Microseconds())

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
		msg := ""
		if len(embeds) <= 0 {
			msg = "This channel doesn't have any kanban group yet."
		}
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Embeds:  &embeds,
		}); err != nil {
			slog.Warn("listKanbanHandler: can't send message about the result", "error", err)
		}

		return nil
	}
}
