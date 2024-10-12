package event_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func natural(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "natural"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Execute CRUD operations on events using natural language.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "content",
				Description: "What do you want to do?",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "event-context-id",
				Description: "The ID of the event using as context for the LLM.",
				Required:    false,
			},
		},
	})
	cmdHandler[id] = naturalHandler(as)
}

func naturalHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction
		if err := ensureCalendarExists(as, s, i); err != nil {
			return err
		}

		// #region - respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("naturalHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// #region - get user params
		content, eventContextID := func() (string, string) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var content, eventContextID string
			if opt, ok := optionMap["content"]; ok {
				content = strings.TrimSpace(opt.StringValue())
			}
			if opt, ok := optionMap["event-context-id"]; ok {
				eventContextID = opt.StringValue()
			}
			return content, eventContextID
		}()
		if content == "" {
			// edit the deferred message
			msg := "Event content is empty."
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about event content is empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - get the event from database if provided event ID
		naturalInputEventContext := new(utils.NaturalInputEventContext)
		naturalInputEventModel := new(model.Event)
		if eventContextID != "" {
			exists, err := as.BunDB.NewSelect().
				Model((*model.Event)(nil)).
				Where("id = ?", eventContextID).
				Where("channel_id = ?", interaction.ChannelID).
				Exists(context.Background())
			if err != nil {
				// edit the deferred message
				msg := fmt.Sprintf("Can't check if event exists\n```\n%s\n```", err.Error())
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("naturalHandler: can't respond about can't check if event exists", "error", err)
				}
				return fmt.Errorf("naturalHandler: can't check if event exists: %w", err)
			}
			if !exists {
				// edit the deferred message
				msg := "Event with the provided event ID not found."
				if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
					Content: &msg,
				}); err != nil {
					slog.Warn("naturalHandler: can't respond about event not found", "error", err)
				}
				return nil
			}
			if err := as.BunDB.NewSelect().
				Model(naturalInputEventModel).
				Where("id = ?", eventContextID).
				Relation("Attendees").
				Scan(context.Background()); err != nil {
				slog.Warn("naturalHandler: can't scan event context", "error", err)
			}
			startTime := time.Unix(0, eventModel.StartDateUnixUTC).UTC()
			startTimeStr := startTime.In(time.Now().Location()).Format("02/01/2006 15:04")
			endTime := time.Unix(0, eventModel.EndDateUnixUTC).UTC()
			endTimeStr := endTime.In(time.Now().Location()).Format("02/01/2006 15:04")
			attendees := make([]string, len(naturalInputEventModel.Attendees))
			for i, attendee := range naturalInputEventModel.Attendees {
				attendees[i] = attendee.Data
			}
			naturalInputEventContext = &utils.NaturalInputEventContext{
				Title:       naturalInputEventModel.Summary,
				Description: naturalInputEventModel.Description,
				Start:       startTimeStr,
				End:         endTimeStr,
				Location:    eventModel.Location,
				URL:         eventModel.URL,
				Location:    naturalInputEventModel.Location,
				URL:         naturalInputEventModel.URL,
				Attendees:   attendees,
			}
		}
		// #endregion

		// #region - process & check success
		naturalOutput, err := as.Natural.NewRequest(content, naturalInputEventContext)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't perform natural request\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't perform natural request", "error", err)
			}
			return fmt.Errorf("naturalHandler: can't perform natural request: %w", err)
		}
		if !naturalOutput.Success {
			// edit the deferred message
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &naturalOutput.Description,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't perform natural request", "error", err)
			}
			return nil
		}
		// #endregion

		switch naturalOutput.Action {
		case utils.NaturalOutputActionTypeCreate:
			return handleActionTypeCreate(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeRead:
			return handleActionTypeRead(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeUpdate:
			return handleActionTypeUpdate(as, s, i, naturalOutput)
		case utils.NaturalOutputActionTypeDelete:
			return handleActionTypeDelete(as, s, i, naturalInputEventContext, naturalOutput)
		default:
			// edit the deferred message
			msg := fmt.Sprintf("Can't create event, invalid action %s from the LLM.", naturalOutput.Action)
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("naturalHandler: can't respond about can't create event, invalid action from the LLM", "error", err)
			}
			return nil
		}
	}
}
