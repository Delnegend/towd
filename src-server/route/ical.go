package route

import (
	"io"
	"log/slog"
	"net/http"
	"towd/src-server/ical"
	"towd/src-server/ical/event"
	"towd/src-server/model"
	"towd/src-server/utils"
)

func Ical(muxer *http.ServeMux, as *utils.AppState) {
	muxer.HandleFunc("GET /ical/{calendar_id}", func(w http.ResponseWriter, r *http.Request) {
		calendarID := r.PathValue("calendar_id")

		// getting the calendar model
		calendalModel := new(model.ExternalCalendar)
		if err := as.BunDB.NewSelect().
			Model(calendalModel).
			Where("id = ?", calendarID).
			Scan(r.Context(), calendalModel); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if calendalModel.Url != "" {
			http.Redirect(w, r, calendalModel.Url, http.StatusFound)
			return
		}

		// turn into ical calendar
		icalCalendar, err := func() (*ical.Calendar, error) {
			icalCalendar := ical.NewCalendar()
			eventModels := make([]model.Event, 0)
			if err := as.BunDB.
				NewSelect().
				Model(&eventModels).
				Where("calendar_id = ?", calendarID).
				Relation("Attendees").
				Scan(r.Context(), &eventModels); err != nil {
				return nil, err
			}
			for _, eventModel := range eventModels {
				undecidedEvent := event.NewUndecidedEvent()
				undecidedEvent.
					SetID(eventModel.ID).
					SetSummary(eventModel.Summary).
					SetDescription(eventModel.Description).
					SetLocation(eventModel.Location).
					SetURL(eventModel.URL).
					SetStartDate(eventModel.StartDateUnixUTC).
					SetEndDate(eventModel.EndDateUnixUTC).
					SetOrganizer(eventModel.Organizer)
				icalEventInter, err := undecidedEvent.DecideEventType()
				if err != nil {
					return nil, err
				}
				if icalEvent, ok := icalEventInter.(event.MasterEvent); ok {
					icalCalendar.AddMasterEvent(icalEvent.GetID(), &icalEvent)
				}
			}
			return &icalCalendar, nil
		}()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// write the ical calendar
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		writer := func(s string) {
			if _, err := io.WriteString(w, s); err != nil {
				slog.Warn("can't write to response", "where", "routes/ical.go", "err", err)
			}
		}
		if err := icalCalendar.ToIcal(writer); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
