package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

func Login(as *utils.AppState) {
	id := "login"
	as.AddAppCmdHandler(id, loginHandler(as))
	as.AddAppCmdInfo(id, &discordgo.ApplicationCommand{
		Name:        id,
		Description: "Get the secret key for the current channel to login to the web client",
	})
}

func loginHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		// #region - respond to the original request
		startTimer := time.Now()
		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			slog.Warn("loginHandler: can't send defer message", "error", err)
			return nil
		}
		as.MetricChans.DiscordSendMessage <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		// #region - get the user ID from interaction
		if i.Member == nil || i.Member.User == nil {
			msg := "Can't get user ID from interaction."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("loginHandler: can't send message about login", "error", err)
			}
			return fmt.Errorf("Login: can't get user ID from interaction")
		}
		// #endregion

		// #region - insert session to DB
		secret := uuid.NewString()
		startTimer = time.Now()
		if _, err := as.BunDB.
			NewInsert().
			Model(&model.Session{
				Secret:           secret,
				Purpose:          model.SESSION_MODEL_PURPOSE_TEMP,
				UserID:           i.Member.User.ID,
				ChannelID:        i.ChannelID,
				CreatedAtUnixUTC: time.Now().UTC().Unix(),
			}).
			Exec(context.Background()); err != nil {
			// edit the deferred message
			msg := fmt.Sprintf("Can't create session\n```\n%s\n```", err.Error())
			if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("loginHandler: can't send message about can't insert session", "error", err)
			}
			return fmt.Errorf("loginHandler: can't insert session: %w", err)
		}
		as.MetricChans.DatabaseWrite <- float64(time.Since(startTimer).Microseconds())
		// #endregion

		msg := fmt.Sprintf("```%s```", secret)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("loginHandler: can't respond about login successful", "error", err)
		}

		return nil
	}
}
