package metric

import (
	"log/slog"
	"time"
	"towd/src-server/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func Init(as *utils.AppState) {
	databaseEmptyRead := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_empty_read_microsec",
		Help: "The latency of an empty database read in microseconds",
	})
	if err := prometheus.Register(databaseEmptyRead); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_empty_read_microsec metric", "error", err)
		}
	} else {
		slog.Debug("towd_database_empty_read_microsec metric registered")
		databaseEmptyRead.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
		ticker := time.NewTicker(as.Config.GetMetricCollectionInterval())
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

	databaseRead := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_read_microsec",
		Help: "The latency of a database read in microseconds",
	})
	if err := prometheus.Register(databaseRead); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_read_microsec metric", "error", err)
		}
	} else {
		slog.Debug("towd_database_read_microsec metric registered")
		databaseRead.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
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
			}
		}
	}()

	databaseWrite := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_database_write_microsec",
		Help: "The latency of a database write in microseconds",
	})
	if err := prometheus.Register(databaseWrite); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_database_write_microsec metric", "error", err)
		}
	} else {
		slog.Debug("towd_database_write_microsec metric registered")
		databaseWrite.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
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
			}
		}
	}()

	discordSendMessage := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_discord_send_message_microsec",
		Help: "The latency of a discord message send in microseconds",
	})
	if err := prometheus.Register(discordSendMessage); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("can't register towd_discord_send_message_microsec metric", "error", err)
		}
	} else {
		slog.Debug("towd_discord_send_message_microsec metric registered")
		discordSendMessage.Set(0)
	}
	go func() {
		gracefulShutdownCh := as.CreateGracefulShutdownChan()
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
			}
		}
	}()

	discordHeartbeatLatency := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "towd_discord_heartbeat_latency_microsec",
		Help: "The latency of a discord heartbeat in microseconds",
	})
	if err := prometheus.Register(discordHeartbeatLatency); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			slog.Error("towd_discord_heartbeat_latency_microsec metric can't register", "error", err)
		}
	} else {
		slog.Debug("towd_discord_heartbeat_latency_microsec metric registered")
		discordHeartbeatLatency.Set(0)
	}
	go func() {
		ticker := time.NewTicker(as.Config.GetMetricCollectionInterval())
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
