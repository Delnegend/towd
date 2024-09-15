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

func createGroup(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "create-group"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Create a Kanban group.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "The name of the group.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = createGroupHandler(as)
}

func createGroupHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// #region - get the group name from param
		groupName, err := func() (string, error) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if opt, ok := optionMap["name"]; ok {
				return strings.TrimSpace(opt.StringValue()), nil
			}
			return "", nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create group\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-group", "content", "can't-create-group", "error", err)
			}
			return nil
		}
		if groupName == "" {
			// edit the deferred message
			msg := "Group content is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-group", "content", "group-content-empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the kanban table of current channel
		kanbanTable, err := func() (*model.KanbanTable, error) {
			kanbanTable := new(model.KanbanTable)
			exists, err := as.BunDB.
				NewSelect().
				Model(kanbanTable).
				Relation("Groups").
				Where("channel_id = ?", interaction.ChannelID).
				Exists(context.Background())
			switch {
			case err != nil:
				return nil, err
			case exists:
				return kanbanTable, nil
			case !exists:
				kanbanTable := model.KanbanTable{
					Name:      "Untitled",
					ChannelID: interaction.ChannelID,
				}
				if _, err := as.BunDB.
					NewInsert().
					Model(&kanbanTable).
					Exec(context.Background()); err != nil {
					return nil, err
				}
				return &kanbanTable, nil
			}
			return nil, nil
		}()
		if err != nil {
			return err
		}
		// #endregion

		// #region - check if group exists
		exists := func() bool {
			for _, group := range kanbanTable.Groups {
				if group.Name == groupName {
					return true
				}
			}
			return false
		}()
		if exists {
			msg := "Group already exists."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "create-group", "content", "group-already-exists", "error", err)
			}
			return nil
		}
		// #endregion

		if _, err := as.BunDB.NewInsert().
			Model(&model.KanbanGroup{
				Name:      groupName,
				ChannelID: interaction.ChannelID,
			}).
			Exec(context.Background()); err != nil {
			return err
		}

		msg := "Group created."
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("can't respond", "handler", "create-group", "content", "group-created", "error", err)
		}

		return nil
	}
}
