package event_handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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
	cmdHandler[id] = createHandler(as)
}

func createHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// respond to the original request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			slog.Warn("can't respond", "handler", "create-event-llm", "content", "deferring", "error", err)
		}

		// get the content
		content, err := func() (string, error) {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if opt, ok := optionMap["content"]; ok {
				return opt.StringValue(), nil
			}
			return "", nil
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create event\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "event-id-required", "error", err)
			}
			return err
		}
		if content == "" {
			// edit the deferred message
			msg := "Event content is empty."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "event-content-empty", "error", err)
			}
			return nil
		}

		// natural text -> static event
		staticEvent, err := func() (*model.StaticEvent, error) {
			staticEvent := new(model.StaticEvent)
			staticEvent.Attendees = new([]string)
			if err := staticEvent.FromNaturalText(context.Background(), as, content); err != nil {
				return nil, err
			}
			return staticEvent, err
		}()
		if err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create event\n```%s```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Error("can't respond", "handler", "modify-event", "content", "event-create-error", "error", err)
			}
			return err
		}

		// ask for confirmation
		if isContinue, timeout, err := func() (bool, bool, error) {
			yesCustomId := "yes-" + staticEvent.ID
			cancelCustomId := "cancel-" + staticEvent.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// edit the deferred message
			msg := "Is this correct?"
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Embeds: &[]*discordgo.MessageEmbed{
					staticEvent.ToDiscordEmbed(context.Background(), as.BunDB),
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
		}(); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't ask for confirmation, can't continue: %s", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "ask-for-confirmation", "error", err)
			}
			return err
		} else if timeout {
			if _, err := s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation."); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "timeout", "error", err)
			}
			return nil
		} else if !isContinue {
			// respond to button request
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Event creation canceled.",
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "cancel", "error", err)
			}
			return nil
		}

		// insert the event
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			// main model
			eventModel := new(model.MasterEvent)
			eventModel.CalendarID = i.GuildID
			eventModel.ID = staticEvent.ID
			eventModel.Summary = staticEvent.Title
			eventModel.Description = staticEvent.Description
			eventModel.StartDate = staticEvent.StartDate
			eventModel.EndDate = staticEvent.EndDate
			eventModel.Location = staticEvent.Location
			eventModel.URL = staticEvent.URL
			eventModel.Organizer = i.Member.User.Username
			eventModel.CreatedAt = time.Now().Unix()
			if err := eventModel.Upsert(ctx, tx); err != nil {
				return err
			}

			// attendee models
			attendeeModels := make([]model.Attendee, len(*staticEvent.Attendees))
			for i, invitee := range *staticEvent.Attendees {
				attendeeModels[i] = model.Attendee{
					EventID: eventModel.ID,
					Data:    invitee,
				}
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
					Content: fmt.Sprintf("Can't create event\n```%s```", err.Error()),
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "modify-event", "content", "event-create-error", "error", err)
			}
			return fmt.Errorf("can't create event: %w", err)
		}

		// respond to button request
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Event created.",
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "modify-event", "content", "last-msg", "error", err)
		}
		return nil
	}
}
