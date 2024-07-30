package auth_handler

import (
	"log/slog"
	"time"
	"towd/src-server/jwt"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

// func Login(as *utils.AppState) {
func login(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "login"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Retrieves the login URL for the app",
	})
	cmdHandler[id] = loginHandler(as)
}

func loginHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		payload := jwt.Payload{
			UserID:   i.Member.User.ID,
			UserName: i.Member.User.Username,
			IssuedAt: time.Now().Unix(),
		}

		token, err := jwt.Encode(payload, as.Config.GetJWTSecret())
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Failed to generate JWT token",
				},
			})
			return err
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "Login into Dashboard",
						URL:   as.Config.GetHostname() + "?token=" + token,
					},
				},
			},
		}); err != nil {
			slog.Warn("can't respond", "handler", "login", "err", err)
		}

		return nil
	}
}
