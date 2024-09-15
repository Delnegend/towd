package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"towd/src-server/handler"
	"towd/src-server/handler/event_handler"
	"towd/src-server/handler/kanban_handler"
	"towd/src-server/model"
	"towd/src-server/routes"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/uptrace/bun"
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
	// There are 2 important things (and others) inside the AppState:
	// - appCmdInfo: a map of all slash commands
	// - appCmdHandler: a map of all slash command handlers
	as := utils.NewAppState()

	if err := model.CreateSchema(as.BunDB); err != nil {
		slog.Error("can't create database schema", "error", err)
		os.Exit(1)
	}

	// injecting interaction handlers into appCmdInfo, appCmdHandler in AppState
	event_handler.Init(as)
	kanban_handler.Init(as)
	handler.ImportCalendar(as)
	handler.Ping(as)

	// tell discordgo how to handle interactions from Discord (w/ appCmdHandler)
	as.DgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
				slog.Warn("can't respond", "error", err.Error())
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

	// open a connection
	if err := as.DgSession.Open(); err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer as.DgSession.Close()

	// tell Discord what commands we have (w/ appCmdInfo)
	if _, err := as.DgSession.ApplicationCommandBulkOverwrite(
		as.Config.GetDiscordClientId(),
		as.Config.GetDiscordGuildID(),
		func() []*discordgo.ApplicationCommand {
			var cmds []*discordgo.ApplicationCommand
			as.IterateAppCmdInfo(func(k string, v *discordgo.ApplicationCommand) {
				cmds = append(cmds, v)
			})
			return cmds
		}()); err != nil {
		slog.Error("can't create slash commands", "error", err.Error())
	}

	// cleanup appCmdInfo from memory
	as.NukeAppCmdInfo()
	runtime.GC()

	// create calendar model for each guild and insert into database
	func() {
		calendarModels := make([]model.Calendar, 0)
		for _, guild := range as.DgSession.State.Guilds {
			calendarModels = append(calendarModels, model.Calendar{
				ID:   guild.ID,
				Name: guild.Name,
			})
		}
		if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewInsert().
				Model(&calendarModels).
				On("CONFLICT (id) DO UPDATE").
				Set("name = EXCLUDED.name").
				Exec(ctx); err != nil {
				return err
			}
			return nil
		}); err != nil {
			slog.Error("can't create calendar model", "error", err)
		}
	}()

	sc := make(chan os.Signal, 1)

	// http server
	go func() {
		muxer := http.NewServeMux()
		routes.Auth(muxer, as)
		routes.Ical(muxer, as)
		routes.SPA(muxer, as, sc)
		if err := http.ListenAndServe(":"+as.Config.GetPort(), muxer); err != nil {
			slog.Error("cannot start HTTP server", "error", err)
			sc <- syscall.SIGTERM
		}
	}()

	slog.Info("number of guilds", "guilds", len(as.DgSession.State.Guilds))
	slog.Info("app is now running, press Ctrl+C to exit")

	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	slog.Info("Gracefully shutting down...")
}
