package metric

import (
	"log/slog"
	"time"
	"towd/src-server/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func databaseEmptyRead(as *utils.AppState, tickerInterval *time.Duration) {
	databaseEmptyRead := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_empty_read_microsec",
		Help: "The latency of an empty database read in microseconds",
	})
	good := true
	if err := prometheus.Register(databaseEmptyRead); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_empty_read_microsec metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_empty_read_microsec metric registered")
		databaseEmptyRead.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		ticker := time.NewTicker(*tickerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseEmptyRead) {
				case true:
					slog.Debug("towd_database_empty_read_microsec metric unregistered")
				case false:
					slog.Warn("towd_database_empty_read_microsec metric not registered")
				}
				return
			case <-ticker.C:
				latency, err := database(as)
				if err != nil {
					slog.Error("can't get database latency", "error", err)
					continue
				}
				databaseEmptyRead.Set(float64(latency.Microseconds()))
			}
		}
	}()
}

func databaseRead(as *utils.AppState, clearTickerInterval *time.Duration) {
	databaseRead := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_read_microsec",
		Help: "The latency of a database read in microseconds",
	})
	good := true
	if err := prometheus.Register(databaseRead); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_read_microsec metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_read_microsec metric registered")
		databaseRead.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseRead) {
				case true:
					slog.Debug("towd_database_read_microsec metric unregistered")
				case false:
					slog.Warn("towd_database_read_microsec metric not registered")
				}
				return
			case latency := <-as.MetricChans.DatabaseRead:
				databaseRead.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				databaseRead.Set(0)
			}
		}
	}()
}

func databaseReadForAuthMiddleware(as *utils.AppState, clearTickerInterval *time.Duration) {
	databaseReadForAuthMiddleware := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_read_microsec_for_auth_middleware",
		Help: "The latency of a database read in microseconds for the auth middleware",
	})
	good := true
	if err := prometheus.Register(databaseReadForAuthMiddleware); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_read_microsec_for_auth_middleware metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_read_microsec_for_auth_middleware metric registered")
		databaseReadForAuthMiddleware.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseReadForAuthMiddleware) {
				case true:
					slog.Debug("towd_database_read_microsec_for_auth_middleware metric unregistered")
				case false:
					slog.Warn("towd_database_read_microsec_for_auth_middleware metric not registered")
				}
				return
			case latency := <-as.MetricChans.DatabaseReadForAuthMiddleware:
				databaseReadForAuthMiddleware.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				databaseReadForAuthMiddleware.Set(0)
			}
		}
	}()
}

func databaseReadForKanbanBoard(as *utils.AppState, clearTickerInterval *time.Duration) {
	databaseReadForKanbanBoard := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_read_microsec_for_kanban_board",
		Help: "The latency of a database read in microseconds for the kanban board",
	})
	good := true
	if err := prometheus.Register(databaseReadForKanbanBoard); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_read_microsec_for_kanban_board metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_read_microsec_for_kanban_board metric registered")
		databaseReadForKanbanBoard.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseReadForKanbanBoard) {
				case true:
					slog.Debug("towd_database_read_microsec_for_kanban_board metric unregistered")
				case false:
					slog.Warn("towd_database_read_microsec_for_kanban_board metric not registered")
				}
				return
			case latency := <-as.MetricChans.DatabaseReadForKanbanBoard:
				databaseReadForKanbanBoard.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				databaseReadForKanbanBoard.Set(0)
			}
		}
	}()
}

func databaseWriteForKanbanBoard(as *utils.AppState, clearTickerInterval *time.Duration) {
	databaseWriteForKanbanBoard := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_write_microsec_for_kanban_board",
		Help: "The latency of a database write in microseconds for the kanban board",
	})
	good := true
	if err := prometheus.Register(databaseWriteForKanbanBoard); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_write_microsec_for_kanban_board metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_write_microsec_for_kanban_board metric registered")
		databaseWriteForKanbanBoard.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseWriteForKanbanBoard) {
				case true:
					slog.Debug("towd_database_write_microsec_for_kanban_board metric unregistered")
				case false:
					slog.Warn("towd_database_write_microsec_for_kanban_board metric not registered")
				}
				return
			case latency := <-as.MetricChans.DatabaseWriteForKanbanBoard:
				databaseWriteForKanbanBoard.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				databaseWriteForKanbanBoard.Set(0)
			}
		}
	}()
}

func databaseWrite(as *utils.AppState, clearTickerInterval *time.Duration) {
	databaseWrite := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_write_microsec",
		Help: "The latency of a database write in microseconds",
	})
	good := true
	if err := prometheus.Register(databaseWrite); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_write_microsec metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_database_write_microsec metric registered")
		databaseWrite.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(databaseWrite) {
				case true:
					slog.Debug("towd_database_write_microsec metric unregistered")
				case false:
					slog.Warn("towd_database_write_microsec metric not registered")
				}
				return
			case latency := <-as.MetricChans.DatabaseWrite:
				databaseWrite.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				databaseWrite.Set(0)
			}
		}
	}()
}

func discordSendMessage(as *utils.AppState, clearTickerInterval *time.Duration) {
	discordSendMessage := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_discord_send_message_microsec",
		Help: "The latency of a discord message send in microseconds",
	})
	good := true
	if err := prometheus.Register(discordSendMessage); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_discord_send_message_microsec metric", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_discord_send_message_microsec metric registered")
		discordSendMessage.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		clearTicker := time.NewTicker(*clearTickerInterval)
		defer clearTicker.Stop()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(discordSendMessage) {
				case true:
					slog.Debug("towd_discord_send_message_microsec metric unregistered")
				case false:
					slog.Warn("towd_discord_send_message_microsec metric not registered")
				}
				return
			case latency := <-as.MetricChans.DiscordSendMessage:
				discordSendMessage.Set(latency)
				clearTicker.Reset(*clearTickerInterval)
			case <-clearTicker.C:
				discordSendMessage.Set(0)
			}
		}
	}()
}

func discordHeartbeatLatency(as *utils.AppState, tickerInterval *time.Duration) {
	discordHeartbeatLatency := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_discord_heartbeat_latency_microsec",
		Help: "The latency of a discord heartbeat in microseconds",
	})
	good := true
	if err := prometheus.Register(discordHeartbeatLatency); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("towd_discord_heartbeat_latency_microsec metric can't register", "error", err)
			good = false
		}
	}
	if good {
		slog.Debug("towd_discord_heartbeat_latency_microsec metric registered")
		discordHeartbeatLatency.Set(0)
	}
	go func() {
		ticker := time.NewTicker(*tickerInterval)
		defer ticker.Stop()
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		for {
			select {
			case <-*gracefulShutdownCh:
				switch prometheus.Unregister(discordHeartbeatLatency) {
				case true:
					slog.Debug("towd_discord_heartbeat_latency_microsec metric unregistered")
				case false:
					slog.Warn("towd_discord_heartbeat_latency_microsec metric not registered")
				}
				return
			case <-ticker.C:
				latency := as.DgSession.HeartbeatLatency().Microseconds()
				discordHeartbeatLatency.Set(float64(latency))
			}
		}
	}()
}

func Init(as *utils.AppState) {
	tickerInterval := as.Config.GetMetricCollectionInterval()
	clearTickerInterval := as.Config.GetMetricCollectionInterval() * 2

	databaseReadForAuthMiddleware(as, &clearTickerInterval)
	databaseReadForKanbanBoard(as, &clearTickerInterval)
	databaseWriteForKanbanBoard(as, &clearTickerInterval)

	databaseEmptyRead(as, &tickerInterval)
	databaseRead(as, &clearTickerInterval)
	databaseWrite(as, &clearTickerInterval)
	discordSendMessage(as, &clearTickerInterval)
	discordHeartbeatLatency(as, &tickerInterval)
}
