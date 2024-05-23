package utils

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"
	"towd/src-server/model"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
)

type AppState struct {
	Config    *Config
	RawDb     *sql.DB
	BunDb     *bun.DB
	DgSession *discordgo.Session
	When      *when.Parser

	// will be send to Discord
	AppCmdInfo map[string]*discordgo.ApplicationCommand
	// handling commands from Discord WSAPI
	AppCmdHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	// same as above but for msg components (buttons, dropdowns, etc)
	MsgComponentHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
	// same as above but for modal (text input)
	ModalHandler map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)

	// calendar event/task queue; since the new temporary handler function and their parent live in the same function scope, we don't need a map to hold them; but for timing out the event, we store them here and remove them both from this map and MsgComponentHandler above
	EventQueue map[uuid.UUID]MsgComponentInfo
}

func NewAppState() *AppState {
	as := &AppState{}

	// init maps
	as.AppCmdInfo = make(map[string]*discordgo.ApplicationCommand)
	as.AppCmdHandler = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	as.MsgComponentHandler = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	as.ModalHandler = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))

	as.EventQueue = make(map[uuid.UUID]MsgComponentInfo)

	go func() {
		for {
			for eventID, eventInfo := range as.EventQueue {
				if time.Since(eventInfo.DateAdded) > 5*time.Minute {
					delete(as.EventQueue, eventID)
					delete(as.MsgComponentHandler, eventID.String())
					slog.Info("event removed from queue", "event", eventInfo)
				}
			}
			time.Sleep(30 * time.Minute)
		}
	}()

	// date parser
	as.When = when.New(nil)
	as.When.Add(en.All...)
	as.When.Add(common.All...)

	// env
	as.Config = NewConfig()

	// database
	var err error
	as.RawDb, err = sql.Open(sqliteshim.ShimName, "./sqlite.db?mode=rwc")
	if err != nil {
		slog.Error("cannot open sqlite database", "error", err)
		os.Exit(1)
	}
	as.RawDb.SetMaxIdleConns(8)

	as.BunDb = bun.NewDB(as.RawDb, sqlitedialect.New())
	as.BunDb.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithVerbose(true),
		bundebug.FromEnv("BUNDEBUG"),
	))

	if _, err = os.Stat("./sqlite.db"); err != nil {
		model.CreateSchema(context.Background(), as.BunDb)
	}

	return as
}
