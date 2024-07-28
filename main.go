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
		slog.Info(err.Error())
	}
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.RFC1123Z,
		}),
	))
}

func main() {
	as := utils.NewAppState()

	if err := model.CreateSchema(as.BunDB); err != nil {
		slog.Error("can't create database schema", "error", err)
		os.Exit(1)
	}

	handler.CreateEventLLM(as)
	handler.CreateEvent(as)
	handler.DeleteEvent(as)
	handler.Events(as)
	handler.ImportCalendar(as)
	handler.ModifyEvent(as)
	handler.Login(as)
	handler.Ping(as)
	handler.Totp(as)

	// tell discordgo how to handle interactions from Discord
	dgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		execute := func(id string) {
			if handler, ok := as.GetAppCmdHandler(id); ok {
				if err := handler(s, i); err != nil {
					slog.Error("handler error", "command", id, "error", err.Error())
				}
				return
			}
			if i == nil || i.Interaction == nil {
				return
			}
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Expired interaction",
				},
			}); err != nil {
				slog.Error("can't respond", "error", err.Error())
			}
			username := func(i *discordgo.InteractionCreate) string {
				if i == nil || i.User == nil {
					return "unknown"
				}
				return i.User.Username
			}(i)
			slog.Debug("someone used an expired interaction", "username", username, "custom_id", id)
		}

		switch i.Type {
		case discordgo.InteractionApplicationCommand: // slash commands
			cmdData := i.ApplicationCommandData()
			execute(cmdData.Name)
		case discordgo.InteractionMessageComponent: // buttons, dropdowns, etc
			componentData := i.MessageComponentData()
			execute(componentData.CustomID)
		case discordgo.InteractionModalSubmit: // modal a.k.a. text input
			modalData := i.ModalSubmitData()
			execute(modalData.CustomID)
		default:
			slog.Error("unknown interaction type", "type", i.Type)
		}
	})

	if dgSession.Open() != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer dgSession.Close()

	// tell Discord what commands we have
	as.IterateAppCmdInfo(func(k string, v *discordgo.ApplicationCommand) {
		if _, err := dgSession.ApplicationCommandCreate(
			dgSession.State.User.ID,
			as.Config.GetDiscordGuildID(),
			v); err != nil {
			slog.Error("cannot create command", "name", k, "error", err)
			return
		}
	})

	slog.Info("number of guilds", "guilds", len(dgSession.State.Guilds))
	slog.Info("app is now running.  Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	slog.Info("Shutting down...")
}
