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

func createItem(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "create-item"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Create a Kanban item.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "content",
				Description: "Describe the event in detail.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "group-name",
				Description: "The group to add the item to.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = createItemHandler(as)
}

func createItemHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("createItemHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// #region - get the content and the target groupName
		content, groupName := func() (string, string) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var content, group string
			if opt, ok := optionMap["content"]; ok {
				content = strings.TrimSpace(opt.StringValue())
			}
			if opt, ok := optionMap["group-name"]; ok {
				group = strings.TrimSpace(opt.StringValue())
			}
			return content, group
		}()
		if content == "" || groupName == "" {
			// edit the deffered message
			msg := "Content and group cannot be empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createItemHandler: can't respond about content and group cannot be empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - check if groupModel with groupName exists
		startTimer = time.Now()
		groupModelExists, err := as.BunDB.
			NewSelect().
			Model((*model.KanbanGroup)(nil)).
			Where("name = ?", groupName).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			// edit the deffered message
			msg := fmt.Sprintf("Can't get group\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createItemHandler: can't respond about can't check if group exists", "error", err)
			}
			return fmt.Errorf("createItemHandler: can't check if group exists: %w", err)
		case !groupModelExists:
			as.MetricChans.DatabaseRead <- float64(time.Since(startTimer).Microseconds())

			// edit the deffered message
			msg := fmt.Sprintf("Group `%s` not found.", groupName)
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createItemHandler: can't respond about group not found", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - insert to db
		startTimer = time.Now()
		if _, err := as.BunDB.NewInsert().
			Model(&model.KanbanItem{
				Content:   content,
				GroupName: groupName,
				ChannelID: interaction.ChannelID,
			}).
			Exec(context.Background()); err != nil {
			// edit the deffered message
			msg := fmt.Sprintf("Can't insert item\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createItemHandler: can't respond about can't insert item into database", "error", err)
			}
			return fmt.Errorf("createItemHandler: can't insert item into database: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		msg := "Item created."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("createItemHandler: can't respond about item creation success", "error", err)
		}

		return nil
	}
}
