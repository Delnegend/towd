package scheduler

import (
	"context"
	"log/slog"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
)

func EventNotify(as *utils.AppState) {
	for {
		time.Sleep(time.Second * 30)

		// get all eventModels starting in 15 minutes from now
		eventModels := make([]model.Event, 0)
		if err := as.BunDB.
			NewSelect().
			Model(&eventModels).
			Relation("Attendees").
			Where("start_date > ?", time.Now().UTC().Unix()).
			Where("start_date < ?", time.Now().UTC().Add(15*time.Minute).Unix()).
			Where("notification_sent = ?", false).
			Scan(context.Background()); err != nil {
			slog.Error("can't get events", "error", err)
			continue
		}

		channelsToEventModels := make(map[string][]*model.Event)
		for _, event := range eventModels {
			channelsToEventModels[event.ChannelID] = append(channelsToEventModels[event.ChannelID], &event)
		}

		for channelID, eventModels := range channelsToEventModels {
			if _, err := as.DgSession.ChannelMessageSendEmbeds(
				channelID,
				func() []*discordgo.MessageEmbed {
					embeds := make([]*discordgo.MessageEmbed, len(eventModels))
					for i, event := range eventModels {
						embeds[i] = event.ToDiscordEmbed()
					}
					return embeds
				}(),
			); err != nil {
				slog.Error("EventNotify: can't send message", "error", err)
				continue
			}

			if _, err := as.BunDB.NewUpdate().
				With("_data", as.BunDB.NewValues(&eventModels)).
				Model((*model.Event)(nil)).
				TableExpr("_data").
				Set("notification_sent = ?", true).
				Where("event.id = _data.id").
				Exec(context.Background()); err != nil {
				slog.Error("EventNotify: can't update notification_sent field", "error", err)
				continue
			}
		}
	}
}
