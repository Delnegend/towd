package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"towd/src-server/ical"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/uptrace/bun"
)

const (
	WORKER_COUNT = 4
)

func CalendarUpdate(as *utils.AppState) {
	for {
		externalCalendars := []model.ExternalCalendar{}
		if err := as.BunDB.
			NewSelect().
			Model(&externalCalendars).
			Where("url LIKE ?", "https://%").
			Scan(context.Background()); err != nil {
			slog.Error("can't get calendars", "error", err)
			time.Sleep(as.Config.GetCalendarUpdateInterval())
			continue
		}
		if len(externalCalendars) == 0 {
			time.Sleep(as.Config.GetCalendarUpdateInterval())
			continue
		}

		jobs := make(chan model.ExternalCalendar, len(externalCalendars))
		var wg sync.WaitGroup

		for range WORKER_COUNT {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for oldExternalCalModel := range jobs {
					calCh := make(chan *ical.Calendar)
					errCh := make(chan error)

					go func() {
						iCalCalendar, err := ical.FromIcalUrl(oldExternalCalModel.Url)
						if err != nil {
							errCh <- err
							return
						}
						calCh <- iCalCalendar
					}()

					select {
					case <-time.After(time.Minute * 5):
						slog.Warn("CalendarUpdate: timed out waiting for calendar to be fetched & parsed")
					case err := <-errCh:
						slog.Warn("CalendarUpdate: can't fetch calendar", "url", oldExternalCalModel.Url, "error", err)
					case icalCal := <-calCh:
						if err := as.BunDB.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
							// remove old calendar model & events
							if _, err := tx.NewDelete().
								Model((*model.Event)(nil)).
								Where("calendar_id = ?", oldExternalCalModel.ID).
								Exec(ctx); err != nil {
								return fmt.Errorf("can't delete old events: %w", err)
							}
							if _, err := tx.NewDelete().
								Model((*model.ExternalCalendar)(nil)).
								Where("id = ?", oldExternalCalModel.ID).
								Exec(ctx); err != nil {
								return fmt.Errorf("can't delete old calendar: %w", err)
							}

							// create new calendar model and insert to DB
							hash, err := utils.GetFileHash(oldExternalCalModel.Url)
							if err != nil {
								return err
							}
							newExternalCalModel := model.ExternalCalendar{
								ID:          icalCal.GetID(),
								ProdID:      icalCal.GetProdID(),
								Name:        icalCal.GetName(),
								Description: icalCal.GetDescription(),
								Url:         oldExternalCalModel.Url,
								Hash:        hash,
								ChannelID:   oldExternalCalModel.ChannelID,
							}
							if _, err := tx.
								NewInsert().
								Model(&newExternalCalModel).
								Exec(ctx); err != nil {
								return err
							}

							eventModels := make([]model.Event, 0)
							for _, event := range icalCal.ToStaticEvents() {
								eventModels = append(eventModels, model.Event{
									ID:               fmt.Sprintf("%s-%s", event.ID, oldExternalCalModel.ChannelID),
									Summary:          event.Title,
									Description:      event.Description,
									Location:         event.Location,
									URL:              event.URL,
									Organizer:        event.Organizer,
									StartDateUnixUTC: event.StartDate,
									EndDateUnixUTC:   event.EndDate,
									CalendarID:       newExternalCalModel.ID,
									ChannelID:        oldExternalCalModel.ChannelID,
								})
							}
							if _, err := tx.NewInsert().
								Model(&eventModels).
								Exec(ctx); err != nil {
								return err
							}
							return nil
						}); err != nil {
							slog.Warn("CalendarUpdate: can't insert calendar", "url", oldExternalCalModel.Url, "error", err)
						}
						close(calCh)
						close(errCh)
					}
				}
			}()
		}

		wg.Wait()

		time.Sleep(as.Config.GetCalendarUpdateInterval())
	}
}
