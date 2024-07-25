package handler

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

func Events(as *utils.AppState) {
	id := "events"
	as.AddAppCmdHandler(id, eventsHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        "events",
		Description: "List all events.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "date",
				Description: "The date to list events for in `DD/MM/YYYY`",
				Required:    false,
			},
		},
	})
}

func eventsHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "events", "content", "deferring", "error", err)
		}

		// #region | parse date and get the start/end date range
		searchDate := "today"
		startStartDateRange := func() time.Time {
			// get the date from params
			responses := i.ApplicationCommandData().Options
			if len(responses) == 0 {
				return time.Now().Truncate(24 * time.Hour)
			}
			if len(responses) == 0 {
				return time.Now().Truncate(24 * time.Hour)
			}
			response := responses[0].StringValue()
			if response == "" {
				return time.Now().Truncate(24 * time.Hour)
			}
			date_ := strings.TrimSpace(response)

			// parse date
			var err error
			date, err := time.Parse("02-01-2006", date_)

			// if can't parse, default to today
			if err != nil {
				return time.Now().Truncate(24 * time.Hour)
			}
			searchDate = fmt.Sprintf("<t:%d:D>", date.Unix())
			return date
		}()
		endStartDateRange := startStartDateRange.Add(24 * time.Hour)
		// #endregion

		// #region | get all events
		staticEvents, err := model.GetStaticEventInRange(context.Background(), as.BunDB, startStartDateRange.Unix(), endStartDateRange.Unix())
		if err != nil {
			msg := fmt.Sprintf("Can't get events in range\n```\n%s```", err.Error())
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			})
			return fmt.Errorf("eventsHandler: %w", err)
		}
		// #endregion

		// #region | compose the message
		embedEvents := []*discordgo.MessageEmbed{}
		for _, event := range *staticEvents {
			fields := []*discordgo.MessageEmbedField{
				{
					Name:   "Start Date",
					Value:  fmt.Sprintf("<t:%d:t>", event.StartDate),
					Inline: true,
				},
				{
					Name:   "End Date",
					Value:  fmt.Sprintf("<t:%d:t>", event.EndDate),
					Inline: true,
				},
			}
			if event.Location != "" {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:  "Location",
					Value: event.Location,
				})
			}
			if event.Attendees != nil {
				if len(*event.Attendees) > 0 {
					fields = append(fields, &discordgo.MessageEmbedField{
						Name:  "Attendees",
						Value: strings.Join(*event.Attendees, ", "),
					})
				}
			}
			embedEvents = append(embedEvents, &discordgo.MessageEmbed{
				Title:       event.Title,
				Description: event.Description,
				Author: &discordgo.MessageEmbedAuthor{
					Name: event.Organizer,
				},
				Fields: fields,
				URL:    event.URL,
				Footer: &discordgo.MessageEmbedFooter{Text: event.ID},
			})
		}

		// prepare the content
		content := func() string {
			if len(embedEvents) == 0 {
				return fmt.Sprintf("No event for %s", searchDate)
			}
			var eventTextSuffix string
			if len(embedEvents) > 1 {
				eventTextSuffix = "s"
			}
			return fmt.Sprintf("There are %d event%s for %s", len(embedEvents), eventTextSuffix, searchDate)
		}()
		// #endregion

		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
			Embeds:  &embedEvents,
		}); err != nil {
			slog.Warn("can't respond", "handler", "events", "content", "events-list", "error", err)
		}

		return nil
	}
}
