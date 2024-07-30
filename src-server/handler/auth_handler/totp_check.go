package auth_handler

import (
	"context"
	"log/slog"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/pquerna/otp/totp"
)

func totpCheck(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "totp-check"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Check if the TOTP code is valid",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "code",
				Description: "Input the TOTP code to verify instead of generating a new one",
				Required:    true,
			},
		},
	})
	cmdHandler[id] = totpCheckHandler(as)
}

func totpCheckHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "totp-check", "content", "deferred", "error", err)
		}

		totpCode := func() string {
			options := i.ApplicationCommandData().Options[0].Options
			optionMap := make(
				map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options),
			)
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var totpCode string
			if opt, ok := optionMap["code"]; ok {
				totpCode = opt.StringValue()
			}
			return totpCode
		}()
		if totpCode == "" {
			msg := "TOTP code is empty."
			if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			}); err != nil {
				slog.Warn("can't respond", "handler", "totp-check", "content", "can't edit", "error", err)
			}
			return nil
		}

		msg := func() string {
			userModel := new(model.User)
			if err := as.BunDB.
				NewSelect().
				Model(userModel).
				Where("id = ?", i.Member.User.ID).
				Scan(context.Background()); err != nil {
				return "You haven't created a TOTP secret yet."
			}
			if totp.Validate(totpCode, userModel.TotpSecret) {
				return "✅  TOTP code is valid."
			}
			return "❌  TOTP code is invalid."
		}()

		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			slog.Warn("can't respond", "handler", "totp", "err", err)
		}

		return nil
	}
}
