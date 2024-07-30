package event_handler

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

// func List(as *utils.AppState) {
func list(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "list"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "List events in a date range.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start",
				Description: "The start of the start date range",
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end",
				Description: "The end of the start date range",
			},
		},
	})
	cmdHandler[id] = listHandler(as)
}

func listHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// response to the original request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "events", "content", "deferring", "error", err)
		}

		// parse date and get the start/end start date range
		searchDate := "today"
		startStartDateRange, endStartDateRange, err := func() (time.Time, time.Time, error) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			var startStartDateRange time.Time
			var endStartDateRange time.Time

			if value, ok := optionMap["start"]; ok {
				parsed, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return time.Time{}, time.Time{}, fmt.Errorf("can't parse start date: %w", err)
				}
				startStartDateRange = parsed.Time.UTC()
			} else {
				startStartDateRange = time.Now().Truncate(24 * time.Hour)
			}
			if value, ok := optionMap["end"]; ok {
				parsed, err := as.When.Parse(value.StringValue(), time.Now())
				if err != nil {
					return time.Time{}, time.Time{}, fmt.Errorf("can't parse end date: %w", err)
				}
				endStartDateRange = parsed.Time.UTC()
			} else {
				endStartDateRange = startStartDateRange.Add(24 * time.Hour)
			}
			return startStartDateRange, endStartDateRange, nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't parse date\n```\n%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "events", "content", "events-list", "error", err)
			}
			return err
		}

		// get all events
		staticEvents, err := model.
			GetStaticEventInRange(
				context.Background(),
				as.BunDB,
				startStartDateRange.Unix(),
				endStartDateRange.Unix(),
			)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't get events in range\n```\n%s```", err.Error())
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			})
			return err
		}

		// compose & send the message
		embeds := []*discordgo.MessageEmbed{}
		for _, event := range *staticEvents {
			fields := []*discordgo.MessageEmbedField{
				{
					Name:   "Start Date",
					Value:  fmt.Sprintf("<t:%d:f>", event.StartDate),
					Inline: true,
				},
				{
					Name:   "End Date",
					Value:  fmt.Sprintf("<t:%d:f>", event.EndDate),
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
			embeds = append(embeds, &discordgo.MessageEmbed{
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
		// edit the deferred message
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: func() *string {
				if len(embeds) == 0 {
					content := fmt.Sprintf("No event for %s", searchDate)
					return &content
				}
				var eventTextSuffix string
				if len(embeds) > 1 {
					eventTextSuffix = "s"
				}
				content := fmt.Sprintf("There are %d event%s for %s", len(embeds), eventTextSuffix, searchDate)
				return &content
			}(),
			Embeds: &embeds,
		}); err != nil {
			slog.Warn("can't respond", "handler", "events", "content", "events-list", "error", err)
		}

		return nil
	}
}