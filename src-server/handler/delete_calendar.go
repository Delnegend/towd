package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

func DeleteCalendar(as *utils.AppState) {
	id := "delete-calendar"
	as.AddAppCmdHandler(id, deleteCalendarHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Delete an external calendar.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "calendar-id",
				Description: "The ID of the calendar to delete.",
				Required:    true,
			},
		},
	})
}

func deleteCalendarHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			slog.Warn("deleteCalendarHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// get the calendar ID
		calendarID := func() string {
			options := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, 0)
			for _, opt := range i.ApplicationCommandData().Options {
				options[opt.Name] = opt
			}
			if opt, ok := options["calendar-id"]; ok {
				return strings.TrimSpace(opt.StringValue())
			}
			return ""
		}()
		if calendarID == "" {
			// edit the deferred message
			msg := "Calendar ID is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("deleteCalendarHandler: can't send message about calendar ID is empty", "error", err)
			}
			return nil
		}

		// check if calendar exists
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.ExternalCalendar)(nil)).
			Where("id = ?", calendarID).
			Where("channel_id = ?", interaction.ChannelID).
			Exists(context.Background())
		switch {
		case err != nil:
			msg := fmt.Sprintf("Can't check if calendar exists\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("deleteCalendarHandler: can't send message about can't check if calendar exists", "error", err)
			}
			return fmt.Errorf("deleteCalendarHandler: can't check if calendar exists: %w", err)
		case !exists:
			msg := "Calendar not found."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("deleteCalendarHandler: can't send message about calendar not found", "error", err)
			}
			return nil
		}
		externalCalendarModel := new(model.ExternalCalendar)
		if err := as.BunDB.
			NewSelect().
			Model(externalCalendarModel).
			Where("id = ?", calendarID).
			Where("channel_id = ?", interaction.ChannelID).
			Scan(context.Background()); err != nil {
			msg := fmt.Sprintf("Can't get calendar\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("deleteCalendarHandler: can't send message about can't get calendar", "error", err)
			}
			return fmt.Errorf("deleteCalendarHandler: can't get calendar: %w", err)
		}

		// delete the calendar
		startTimer = time.Now()
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewDelete().
				Model((*model.ExternalCalendar)(nil)).
				Where("id = ?", calendarID).
				Where("channel_id = ?", interaction.ChannelID).
				Exec(ctx); err != nil {
				return fmt.Errorf("transaction: can't delete calendar model: %w", err)
			}
			if _, err := tx.NewDelete().
				Model((*model.Event)(nil)).
				Where("calendar_id = ?", calendarID).
				Where("channel_id = ?", interaction.ChannelID).
				Exec(ctx); err != nil {
				return fmt.Errorf("transaction: can't delete event models: %w", err)
			}
			return nil
		}); err != nil {
			msg := fmt.Sprintf("Can't delete calendar.\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("deleteCalendarHandler: can't respond about can't delete calendar", "error", err)
			}
			return fmt.Errorf("deleteCalendarHandler: can't delete calendar: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())

		// edit the deferred message (ephemeral)
		msg := fmt.Sprintf("Calendar [%s](%s) deleted.", externalCalendarModel.Name, externalCalendarModel.Url)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("deleteCalendarHandler: can't respond about calendar deletion success", "error", err)
		}

		// announce the calendar deletion
		msg = func() string {
			var sb strings.Builder
			if i != nil && i.Member != nil && i.Member.User != nil {
				sb.WriteString(fmt.Sprintf("<@%s> deleted", i.Member.User.ID))
			} else {
				sb.WriteString("Deleted")
			}
			sb.WriteString(fmt.Sprintf(" calendar [%s](%s).", externalCalendarModel.Name, externalCalendarModel.Url))
			return sb.String()
		}()
		if _, err := s.ChannelMessageSend(interaction.ChannelID, msg); err != nil {
			slog.Warn("deleteCalendarHandler: can't send message about calendar deletion success", "error", err)
		}

		return nil
	}
}
