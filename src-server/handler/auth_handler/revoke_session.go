package auth_handler

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func revokeSession(as *utils.AppState, cmdInfo *[]*discordgo.ApplicationCommandOption, cmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	id := "revoke-session"
	*cmdInfo = append(*cmdInfo, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        id,
		Description: "Invalidate previous session",
	})
	cmdHandler[id] = revokeSessionHandler(as)
}

func revokeSessionHandler(as *utils.AppState) func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		interaction := i.Interaction

		if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			return fmt.Errorf("can't send defer message: %w", err)
		}

		// get all sessions for user to revoke
		sessionTokens := make([]model.SessionToken, 0)
		if err := as.BunDB.
			NewSelect().
			Model(&sessionTokens).
			Where("user_id = ?", i.Member.User.ID).
			Scan(context.Background(), &sessionTokens); err != nil {
			return fmt.Errorf("can't get all sessions for user to revoke: %w", err)
		}
		if len(sessionTokens) == 0 {
			return fmt.Errorf("no sessions to revoke")
		}

		// send list of sessions to revoke
		msg := fmt.Sprintf("Available sessions: %d", len(sessionTokens))
		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Embeds: func() *[]*discordgo.MessageEmbed {
				embeds := make([]*discordgo.MessageEmbed, 0)
				for i, sessionToken := range sessionTokens {
					embeds = append(embeds, &discordgo.MessageEmbed{
						Title: "Session revoked",
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:   "Index",
								Value:  fmt.Sprintf("%d", i),
								Inline: true,
							},
							{
								Name:   "Created At",
								Value:  fmt.Sprintf("<t:%d:f>", sessionToken.CreatedAtUnix),
								Inline: true,
							},
							{
								Name:   "IP Address",
								Value:  sessionToken.IpAddress,
								Inline: true,
							},
							{
								Name:   "User Agent",
								Value:  sessionToken.UserAgent,
								Inline: false,
							},
						},
					})
				}
				return &embeds
			}(),
			Components: func() *[]discordgo.MessageComponent {
				// create n rows, each row has max 5 buttons
				rows := make([]discordgo.ActionsRow, 0)
				for i := 0; i < len(sessionTokens); i += 5 {
					buttons := make([]discordgo.MessageComponent, 0)
					for j := i; j < i+5 && j < len(sessionTokens); j++ {
						buttons = append(buttons, discordgo.Button{
							Label:    fmt.Sprintf("%d", j),
							Style:    discordgo.DangerButton,
							CustomID: fmt.Sprintf("revoke-session-%d", j),
						})
					}
					rows = append(rows, discordgo.ActionsRow{Components: buttons})
				}
				components := []discordgo.MessageComponent{}
				for _, row := range rows {
					components = append(components, row)
				}
				return &components
			}(),
		}); err != nil {
			return fmt.Errorf("can't list sessions for user to revoke: %w", err)
		}

		revokeSessionSignalCh := make(chan int)
		defer close(revokeSessionSignalCh)

		for i := range sessionTokens {
			as.AddAppCmdHandler(fmt.Sprintf("revoke-session-%d", i), func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
				interaction = i.Interaction
				index, err := strconv.Atoi(i.Interaction.MessageComponentData().CustomID[len("revoke-session-"):])
				if err != nil {
					return fmt.Errorf("can't parse index: %w", err)
				}
				revokeSessionSignalCh <- index
				return nil
			})
			defer as.RemoveAppCmdHandler(fmt.Sprintf("revoke-session-%d", i))
		}

		select {
		case <-time.After(time.Minute * 2):
			return nil
		case index := <-revokeSessionSignalCh:
			if index < 0 || index >= len(sessionTokens) {
				return fmt.Errorf("invalid index: %d", index)
			}
			sessionToken := sessionTokens[index]
			if _, err := as.
				BunDB.
				NewDelete().
				Model((*model.SessionToken)(nil)).
				Where("secret = ?", sessionToken.Secret).Exec(context.Background()); err != nil {
				return fmt.Errorf("can't delete session token: %w", err)
			}
			if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Session revoked.",
				},
			}); err != nil {
				return fmt.Errorf("can't respond to revoke session button: %w", err)
			}
		}

		return nil
	}
}
