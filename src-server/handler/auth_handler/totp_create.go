package auth_handler

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"log/slog"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/pquerna/otp/totp"
)

func totpCreate(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "totp-create"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Generates a TOTP secret",
	})
	cmdHandler[id] = totpCreateHandler(as)
}

func totpCreateHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		totpCode := func() string {
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(i.ApplicationCommandData().Options))
			for _, opt := range i.ApplicationCommandData().Options {
				optionMap[opt.Name] = opt
			}
			var totpCode string
			if opt, ok := optionMap["code"]; ok {
				totpCode = opt.StringValue()
			}
			return totpCode
		}()

		switch totpCode == "" {
		case true: // generate new TOTP secret
			// prepare IDs, channels
			confirmCustomID := "confirm-totp-" + i.Member.User.ID
			cancelCustomID := "cancel-totp-" + i.Member.User.ID
			confirmCh := make(chan struct{})
			cancelCh := make(chan struct{})
			defer close(confirmCh)
			defer close(cancelCh)

			// send confirm button
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "This will overwrite your current TOTP secret. Are you sure?",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									CustomID: confirmCustomID,
									Label:    "Yes",
									Style:    discordgo.PrimaryButton,
								}, discordgo.Button{
									CustomID: cancelCustomID,
									Label:    "No",
									Style:    discordgo.SecondaryButton,
								},
							},
						},
					},
				},
			}); err != nil {
				slog.Warn("can't respond", "handler", "totp", "content", "confirming", "error", err)
			}

			// add handlers for buttons
			var buttonInteraction *discordgo.Interaction
			as.AddAppCmdHandler(confirmCustomID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				buttonInteraction = i.Interaction
				confirmCh <- struct{}{}
				return nil
			})
			as.AddAppCmdHandler(cancelCustomID, func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				buttonInteraction = i.Interaction
				cancelCh <- struct{}{}
				return nil
			})
			defer as.RemoveAppCmdHandler(confirmCustomID)
			defer as.RemoveAppCmdHandler(cancelCustomID)

			// wait for user to confirm or cancel
			select {
			case <-cancelCh:
				s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "Canceled.",
					},
				})
				return nil
			case <-confirmCh:
			case <-time.After(time.Minute * 2):
				s.ChannelMessageSend(i.ChannelID, "Timed out waiting for confirmation.")
				return nil
			}

			err := func() error {
				// create new TOTP secret
				key, err := totp.Generate(totp.GenerateOpts{
					Issuer:      "towd",
					AccountName: i.Member.User.Username,
				})
				if err != nil {
					return err
				}

				// create new user model
				userModel := new(model.User)
				userModel.ID = i.Member.User.ID
				userModel.Username = i.Member.User.Username
				userModel.TotpSecret = key.Secret()
				if err := userModel.Upsert(context.Background(), as.BunDB); err != nil {
					return fmt.Errorf("can't upsert user: %w", err)
				}

				// turn TOTP URL QR image into image buffer
				img, err := key.Image(200, 200)
				if err != nil {
					return fmt.Errorf("can't generate QR code: %w", err)
				}
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					return fmt.Errorf("can't write QR code to buffer: %w", err)
				}

				// send QR code to user
				if err := s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags: discordgo.MessageFlagsEphemeral,
						Files: []*discordgo.File{{Name: "qr.png", Reader: bytes.NewReader(buf.Bytes())}},
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "totp", "content", "can't respond", "err", err)
				}

				return nil
			}()
			if err != nil {
				if err := s.InteractionRespond(buttonInteraction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "Can't generate TOTP secret",
					},
				}); err != nil {
					slog.Warn("can't respond", "handler", "totp", "err", err.Error())
				}
				return fmt.Errorf("totpHandler: %w", err)
			}
		case false: // validate TOTP code

		}
		return nil
	}
}
