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

		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("createGroupHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		if err := ensureKanbantableExists(as, s, i); err != nil {
			return err
		}

		// #region - get the group name from param
		groupName := func() string {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if opt, ok := optionMap["name"]; ok {
				return strings.TrimSpace(opt.StringValue())
			}
			return ""
		}()
		if groupName == "" {
			// edit the deferred message
			msg := "Group content is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createGroupHandler: can't respond about group content is empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the kanban table of current channel
		kanbanTable := new(model.KanbanTable)
		exists, err := as.BunDB.
			NewSelect().
			Model(kanbanTable).
			Relation("Groups").
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			msg := fmt.Sprintf("Can't check if kanban table exists\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createGroupHandler: can't respond about can't check if kanban table exists", "error", err)
			}
			return fmt.Errorf("createGroupHandler: can't check if kanban table exists: %w", err)
		case !exists:
			kanbanTable := model.KanbanTable{
				Name:      "Untitled",
				ChannelID: interaction.ChannelID,
			}
			if _, err := as.BunDB.
				NewInsert().
				Model(&kanbanTable).
				Exec(context.Background()); err != nil {
				msg := fmt.Sprintf("Kanban table not exist but can't create new one\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("createGroupHandler: can't respond about can't create kanban table", "error", err)
				}
				return fmt.Errorf("createGroupHandler: kanban table not exist, can't create new one: %w", err)
			}
		}
		// #endregion

		// #region - check if group exists
		startTimer = time.Now()
		groupModelExists, err := as.BunDB.
			NewSelect().
			Model((*model.KanbanGroup)(nil)).
			Where("name = ?", groupName).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		as.MetricChans.DatabaseRead <- float64(time.Since(startTimer).Microseconds())
		switch {
		case err != nil:
			msg := fmt.Sprintf("Can't check if group exists\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createGroupHandler: can't respond about can't check if group exists", "error", err)
			}
			return fmt.Errorf("createGroupHandler: can't check if group exists: %w", err)
		case groupModelExists:
			msg := "Group already exists."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createGroupHandler: can't respond about group already exists", "error", err)
			}
			return nil
		}
		// #endregion

		// insert group to db
		startTimer = time.Now()
		if _, err := as.BunDB.NewInsert().
			Model(&model.KanbanGroup{
				Name:      groupName,
				ChannelID: interaction.ChannelID,
			}).
			Exec(context.Background()); err != nil {
			msg := fmt.Sprintf("Can't insert new group\n```\n%s\n```", err.Error())
			if _, err2 := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err2 != nil {
				slog.Warn("createGroupHandler: can't respond about can't insert new group", "error", err)
			}
			return fmt.Errorf("createGroupHandler: can't insert new group: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())

		msg := fmt.Sprintf("Group `%s` created.", groupName)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("createGroupHandler: can't respond about group creation success", "error", err)
		}

		return nil
	}
}
