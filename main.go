package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"towd/src-server/handler"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
		panic(err)
	}
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.RFC1123Z,
		}),
	))
}

func main() {
	appState := utils.NewAppState()

	handler.Ping(appState)
	handler.Events(appState)
	handler.CreateEvent(appState)

	// init discord session
	var err error
	appState.DgSession, err = discordgo.New("Bot " + appState.Config.DiscordBotToken())
	if err != nil {
		slog.Error("error creating Discord session", "error", err)
		return
	}
	appState.DgSession.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("logged in as", "username", s.State.User.Username, "discriminator", s.State.User.Discriminator)
	})

	// assign handlers to the instance
	appState.DgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		// all the slash commands
		case discordgo.InteractionApplicationCommand:
			cmdData := i.ApplicationCommandData()
			if handler, ok := appState.AppCmdHandler[cmdData.Name]; ok {
				handler(s, i)
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "WIP command",
				},
			})
			slog.Warn("no handler for command", "name", cmdData.Name, "options", cmdData.Options)

		// interactive elements like buttons, dropdowns, etc
		case discordgo.InteractionMessageComponent:
			componentData := i.MessageComponentData()
			if handler, ok := appState.MsgComponentHandler[componentData.CustomID]; ok {
				handler(s, i)
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Action expired",
				},
			})
			slog.Debug("someone used an expired action", "username", i.User.Username, "custom_id", componentData.CustomID)

		// modal (text input)
		case discordgo.InteractionModalSubmit:
			modalData := i.ModalSubmitData()
			if handler, ok := appState.ModalHandler[modalData.CustomID]; ok {
				handler(s, i)
				return
			}
			s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
				Content: "Action expired",
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			slog.Debug("someone submitted an expired modal", "username", i.User.Username, "custom_id", modalData.CustomID)
		}
	})

	if appState.DgSession.Open() != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer appState.DgSession.Close()

	// send commands info to Discord backend
	for k, v := range appState.AppCmdInfo {
		if _, err := appState.DgSession.ApplicationCommandCreate(
			appState.DgSession.State.User.ID,
			appState.Config.DiscordGuildID(),
			v); err != nil {
			slog.Error("cannot create command", "name", k, "error", err)
			return
		}
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	slog.Info("Shutting down...")
}
