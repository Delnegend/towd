package auth_handler

import (
	"context"
	"fmt"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/pquerna/otp/totp"
)

// func Login(as *utils.AppState) {
func totpNew(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "totp-new"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Get the current TOTP code",
	})
	cmdHandler[id] = totpNewHandler(as)
}

func totpNewHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			return fmt.Errorf("can't respond defer message to user: %w", err)
		}

		//get the appropiate user model
		userModel, err := func() (*model.User, error) {
			userModel := new(model.User)

			// check if user exists
			exist, err := as.BunDB.
				NewSelect().
				Model((*model.User)(nil)).
				Where("id = ?", i.Member.User.ID).
				Exists(context.Background())
			if err != nil {
				return nil, fmt.Errorf("can't check if user exists: %w", err)
			}

			if exist {
				if err := as.BunDB.
					NewSelect().
					Model(userModel).
					Scan(context.Background()); err != nil {
					return nil, fmt.Errorf("can't get user: %w", err)
				}
				return userModel, nil
			}

			// new key
			key, err := totp.Generate(totp.GenerateOpts{
				Issuer:      "towd",
				AccountName: i.Member.User.Username,
			})
			if err != nil {
				return nil, fmt.Errorf("can't generate totp secret: %w", err)
			}

			// new model
			userModel = &model.User{
				ID:         i.Member.User.ID,
				Username:   i.Member.User.Username,
				TotpSecret: key.Secret(),
			}

			// insert model
			if _, err := as.BunDB.NewInsert().
				Model(userModel).
				Exec(context.Background()); err != nil {
				return nil, fmt.Errorf("can't insert user: %w", err)
			}

			return userModel, nil
		}()
		if err != nil {
			return err
		}

		// generate current TOTP code
		totpCode, err := totp.GenerateCode(userModel.TotpSecret, time.Now())
		if err != nil {
			return fmt.Errorf("can't generate totp code: %w", err)
		}

		msg := fmt.Sprintf("Your current TOTP code is `%s`", totpCode)
		if _, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &msg,
		}); err != nil {
			return fmt.Errorf("can't edit message: %w", err)
		}

		return nil
	}
}
