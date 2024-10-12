package utils

import (
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

type AppState struct {
	Config *Config

	BunDB     *bun.DB
	DgSession *discordgo.Session
	When      *when.Parser

	startedAt time.Time

	// will be send to Discord
	appCmdInfo      map[string]*discordgo.ApplicationCommand
	appCmdInfoMutex sync.RWMutex
	// handling commands from Discord WSAPI
	appCmdHandler      map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error
	appCmdHandlerMutex sync.RWMutex

	MetricChans *Metric

	// send a signal into this channel to perform graceful shutdown
	AppCloseSignalChan chan os.Signal
	// any long-running task that needs to be gracefully shutdown should
	// add a channel to this list, once a app close signal above is triggered,
	// all channels in this list will be also sent a signal
	gracefulShutdownChans      []*chan struct{}
	gracefulShutdownChansMutex sync.RWMutex

	Natural Natural
}

func NewAppState() *AppState {
	startedAt := time.Now()
	config := NewConfig()
	return &AppState{
		Config: config,

		BunDB: func() *bun.DB {
			if _, err := os.Stat("./data"); os.IsNotExist(err) {
				if err := os.Mkdir("./data", 0755); err != nil {
					slog.Error("cannot create data directory", "error", err)
					os.Exit(1)
				}
			}

			rawDB, err := sql.Open(sqliteshim.ShimName, "./data/db.sqlite?mode=rwc")
			if err != nil {
				slog.Error("cannot open sqlite database", "error", err)
				os.Exit(1)
			}
			if _, err := rawDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
				slog.Error("cannot set WAL mode", "error", err)
				os.Exit(1)
			}
			return bun.NewDB(rawDB, sqlitedialect.New())
		}(),
		DgSession: func() *discordgo.Session {
			dgSession, err := discordgo.New("Bot " + config.GetDiscordAppToken())
			if err != nil {
				slog.Error("can't create Discord session", "error", err)
				os.Exit(1)
			}
			return dgSession
		}(),
		When: func() *when.Parser {
			w := when.New(nil)
			w.Add(en.All...)
			w.Add(common.All...)
			return w
		}(),

		startedAt: startedAt,

		appCmdInfo:         make(map[string]*discordgo.ApplicationCommand),
		appCmdInfoMutex:    sync.RWMutex{},
		appCmdHandler:      make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error),
		appCmdHandlerMutex: sync.RWMutex{},

		MetricChans: NewMetric(),

		AppCloseSignalChan:         make(chan os.Signal, 1),
		gracefulShutdownChans:      make([]*chan struct{}, 0),
		gracefulShutdownChansMutex: sync.RWMutex{},

		Natural: func() Natural {
			natural, err := InitNatural(config)
			if err != nil {
				slog.Error("cannot initialize Natural", "error", err)
				os.Exit(1)
			}
			return natural
		}(),
	}
}

// GetUptime returns the uptime of the app.
func (as *AppState) GetUptime() time.Duration {
	return time.Since(as.startedAt).Truncate(time.Second)
}

// AddAppCmdInfo adds a slash command info to the AppState.
func (as *AppState) AddAppCmdInfo(id string, info *discordgo.ApplicationCommand) {
	as.appCmdInfoMutex.Lock()
	defer as.appCmdInfoMutex.Unlock()
	as.appCmdInfo[id] = info
}

// IterateAppCmdInfo iterates over all slash command info in the AppState.
func (as *AppState) IterateAppCmdInfo(f func(k string, v *discordgo.ApplicationCommand)) {
	as.appCmdInfoMutex.RLock()
	defer as.appCmdInfoMutex.RUnlock()
	for k, v := range as.appCmdInfo {
		f(k, v)
	}
}

// NukeAppCmdInfo re-initializes the slash command info
// in the AppState in order to free up memory.
func (as *AppState) NukeAppCmdInfo() {
	as.appCmdInfoMutex.Lock()
	defer as.appCmdInfoMutex.Unlock()
	as.appCmdInfo = make(map[string]*discordgo.ApplicationCommand)
}

// AddAppCmdHandler adds a slash command handler to the AppState.
func (as *AppState) AddAppCmdHandler(id string, handler func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	as.appCmdHandlerMutex.Lock()
	defer as.appCmdHandlerMutex.Unlock()
	as.appCmdHandler[id] = handler
}

// GetAppCmdHandler gets a slash command handler from the AppState.
func (as *AppState) GetAppCmdHandler(id string) (func(s *discordgo.Session, i *discordgo.InteractionCreate) error, bool) {
	as.appCmdHandlerMutex.RLock()
	defer as.appCmdHandlerMutex.RUnlock()
	appCmdHandler, ok := as.appCmdHandler[id]
	return appCmdHandler, ok
}

// RemoveAppCmdHandler removes a slash command handler from the AppState.
func (as *AppState) RemoveAppCmdHandler(id string) {
	as.appCmdHandlerMutex.Lock()
	defer as.appCmdHandlerMutex.Unlock()
	delete(as.appCmdHandler, id)
}

// CreateGracefulShutdownChan creates a channel and adds it to the AppState.
func (as *AppState) CreateGracefulShutdownChan() *chan struct{} {
	ch := make(chan struct{})
	as.gracefulShutdownChansMutex.Lock()
	defer as.gracefulShutdownChansMutex.Unlock()
	as.gracefulShutdownChans = append(as.gracefulShutdownChans, &ch)
	return &ch
}

// RemoveGracefulShutdownChan removes a channel from the AppState.
func (as *AppState) RemoveGracefulShutdownChan(ch *chan struct{}) {
	as.gracefulShutdownChansMutex.Lock()
	defer as.gracefulShutdownChansMutex.Unlock()
	for i, c := range as.gracefulShutdownChans {
		if c == ch {
			as.gracefulShutdownChans = append(as.gracefulShutdownChans[:i], as.gracefulShutdownChans[i+1:]...)
			return
		}
	}
}

func (as *AppState) GracefulShutdown() {
	as.gracefulShutdownChansMutex.Lock()
	defer as.gracefulShutdownChansMutex.Unlock()
	for _, ch := range as.gracefulShutdownChans {
		*ch <- struct{}{}
	}
}
