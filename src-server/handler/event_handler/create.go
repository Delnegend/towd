package event_handler

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

func create(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "create"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Create an event.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "content",
				Description: "Describe the event in detail.",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = createEventHandler(as)
}

func createEventHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction
		if err := ensureCalendarExists(as, s, i); err != nil {
			return err
		}

		// respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("createEventHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())

		// #region - get the content
		content := func() string {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if opt, ok := optionMap["content"]; ok {
				return strings.TrimSpace(opt.StringValue())
			}
			return ""
		}()
		if content == "" {
			// edit the deferred message
			msg := "Event content is empty."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createEventHandler: can't respond about event content is empty", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - natural text -> event model
		eventModel := new(model.Event)
		eventModel.CalendarID = i.ChannelID
		eventModel.ChannelID = i.ChannelID
		if i.Member != nil && i.Member.User != nil {
			eventModel.Organizer = i.Member.User.Username
		}
		attendeeModels, err := eventModel.FromNaturalText(context.Background(), as, content)
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create event\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createEventHandler: can't respond about can't parse event data", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - ask for confirmation
		isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + eventModel.ID
			cancelCustomId := "cancel-" + eventModel.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// edit the deferred message
			msg := "Is this correct?"
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds: &[]*discordgo.MessageEmbed{
					eventModel.ToDiscordEmbed(context.Background(), as.BunDB),
				},
				Components: &[]discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Yes",
								Style:    discordgo.SuccessButton,
								CustomID: yesCustomId,
							},
							discordgo.Button{
								Label:    "Cancel",
								Style:    discordgo.DangerButton,
								CustomID: cancelCustomId,
							},
						},
					},
				},
			}); err != nil {
				return false, false, fmt.Errorf("can't ask for confirmation, can't continue: %w", err)
			}

			// add handlers for buttons
			as.AddAppCmdHandler(yesCustomId, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				confirmCh <- struct{}{}
				return nil
			})
			as.AddAppCmdHandler(cancelCustomId, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				cancelCh <- struct{}{}
				return nil
			})
			defer as.RemoveAppCmdHandler(yesCustomId)
			defer as.RemoveAppCmdHandler(cancelCustomId)

			select {
			case <-cancelCh:
				return false, false, nil
			case <-confirmCh:
				return true, false, nil
			case <-time.After(time.Minute * 2):
				return false, false, fmt.Errorf("timeout waiting for confirmation")
			}
		}()
		switch {
		case err != nil:
			// edit the deferred message
			msg := fmt.Sprintf("Can't ask for confirmation, can't continue: %s", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("createEventHandler: can't respond about can't ask for confirmation", "error", err)
			}
			return nil
		case timeout:
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("createEventHandler: can't send timed out waiting for confirmation message", "error", err)
			}
			return nil
		case !isContinue:
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event creation canceled.",
				},
			}); err != nil {
				slog.Warn("createEventHandler: can't respond about event creation canceled", "error", err)
			}
			return nil
		}
		// #endregion

		// #region - insert to db
		startTimer = time.Now()
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if err := eventModel.Upsert(ctx, tx); err != nil {
				return err
			}
			if len(attendeeModels) > 0 {
				if _, err := tx.NewInsert().
					Model(&attendeeModels).
					Exec(ctx); err != nil {
					return fmt.Errorf("can't insert attendees: %w", err)
				}
			}
			return nil
		}); err != nil {
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Can't insert event to database\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("createEventHandler: can't respond about can't insert event to database", "error", err)
			}
			return fmt.Errorf("createEventHandler: can't insert event to database: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// respond to button request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event created.",
			},
		}); err != nil {
			slog.Warn("createEventHandler: can't respond about event creation success", "error", err)
		}
		return nil
	}
}
