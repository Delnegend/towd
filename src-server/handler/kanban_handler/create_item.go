package kanban_handler

import (
	"context"
	"fmt"
	"log/slog"
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
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "kanban-create", "content", "deferring", "error", err)
		}

		// #region - get the content and the target groupName
		content, groupName, err := func() (string, string, error) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var content, group string
			if opt, ok := optionMap["content"]; ok {
				content = opt.StringValue()
			}
			if opt, ok := optionMap["group-name"]; ok {
				group = opt.StringValue()
			}
			return content, group, nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create kanban item\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
			}
			return err
		}
		if content == "" || groupName == "" {
			// edit the deffered message
			msg := "Content and group cannot be empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the group model
		groupModel := new(model.KanbanGroup)
		if err := as.BunDB.
			NewSelect().
			Model(groupModel).
			Where("name = ?", groupName).
			Scan(context.Background(), groupModel); err != nil {
			// edit the deffered message
			msg := fmt.Sprintf("Can't get group\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
			}
			return err
		}
		if groupModel.ChannelID != interaction.ChannelID {
			// edit the deffered message
			msg := "You can't create an item in this group."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
			}
			return nil
		}
		// #endregion

		// #region - insert to db
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
				slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
				return err
			}
			return nil
		}
		// #endregion

		msg := "Item created."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("can't respond", "handler", "kanban-create", "content", msg, "error", err)
		}

		return nil
	}
}
