package routes

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"towd/src-server/ical"
	"towd/src-server/ical/event"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/xyedo/rrule"
)

func Ical(muxer *http.ServeMux, as *utils.AppState) {
	muxer.HandleFunc("GET /ical/{calendar_id}", func(w http.ResponseWriter, r *http.Request) {
		calendarID := r.PathValue("calendar_id")

		// getting the calendar model
		calendalModel := new(model.Calendar)
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
			masterEventModels := make([]model.MasterEvent, 0)
			if err := as.BunDB.
				NewSelect().
				Model(&masterEventModels).
				Where("calendar_id = ?", calendarID).
				Scan(r.Context(), &masterEventModels); err != nil {
				return nil, err
			}
			slog.Info("found master events", "count", len(masterEventModels))
			for _, masterEventModel := range masterEventModels {
				undecidedIcalEvent := event.NewUndecidedEvent()
				undecidedIcalEvent.
					SetID(masterEventModel.ID).
					SetSummary(masterEventModel.Summary).
					SetDescription(masterEventModel.Description).
					SetLocation(masterEventModel.Location).
					SetURL(masterEventModel.URL).
					SetStartDate(masterEventModel.StartDate).
					SetEndDate(masterEventModel.EndDate).
					SetOrganizer(masterEventModel.Organizer).
					SetSequence(masterEventModel.Sequence).
					SetRRuleSet(func() *rrule.Set {
						if masterEventModel.RRule == "" {
							return nil
						}
						rruleSet, err := rrule.StrToRRuleSet(masterEventModel.RRule)
						if err != nil {
							slog.Warn("can't parse rrule", "where", "routes/ical.go", "rrule", masterEventModel.RRule)
							return nil
						}
						return rruleSet
					}()).
					SetExDate(func() []int64 {
						if len(masterEventModel.ExDate) == 0 {
							return []int64{}
						}
						dates := make([]int64, 0)
						for _, exDate := range strings.Split(masterEventModel.ExDate, ",") {
							parsedDate, err := strconv.ParseInt(exDate, 10, 64)
							if err != nil {
								slog.Warn("can't parse exdate", "where", "routes/ical.go", "exdate", exDate)
								continue
							}
							dates = append(dates, parsedDate)
						}
						return dates
					}()).
					SetRDate(func() []int64 {
						if len(masterEventModel.RDate) == 0 {
							return []int64{}
						}
						dates := make([]int64, 0)
						for _, rDate := range strings.Split(masterEventModel.RDate, ",") {
							parsedDate, err := strconv.ParseInt(rDate, 10, 64)
							if err != nil {
								slog.Warn("can't parse rdate", "where", "routes/ical.go", "rdate", rDate)
								continue
							}
							dates = append(dates, parsedDate)
						}
						return dates
					}())

				decidedEvent, err := undecidedIcalEvent.DecideEventType()
				if err != nil {
					return nil, err
				}

				masterIcalEvent, ok := decidedEvent.(event.MasterEvent)
				switch ok {
				case true:
					icalCalendar.AddMasterEvent(masterIcalEvent.GetID(), &masterIcalEvent)
				case false:
					slog.Warn("can't cast event to master event", "where", "routes/ical.go", "eventID", undecidedIcalEvent.GetID())
					continue
				}

				childEventModels := make([]model.ChildEvent, 0)
				if err := as.BunDB.
					NewSelect().
					Model(&childEventModels).
					Where("id = ?", calendarID).
					Scan(r.Context(), &childEventModels); err != nil {
					return nil, err
				}
				for _, childEventModel := range childEventModels {
					childEvent := event.NewUndecidedEvent()
					childEvent.
						SetID(childEventModel.ID).
						SetRecurrenceID(childEventModel.RecurrenceID).
						SetSummary(childEventModel.Summary).
						SetDescription(childEventModel.Description).
						SetLocation(childEventModel.Location).
						SetURL(childEventModel.URL).
						SetOrganizer(childEventModel.Organizer).
						SetStartDate(childEventModel.StartDate).
						SetEndDate(childEventModel.EndDate).
						SetSequence(childEventModel.Sequence)

					decidedEvent, err := childEvent.DecideEventType()
					if err != nil {
						return nil, err
					}

					if childIcalEvent, ok := decidedEvent.(event.ChildEvent); ok {
						masterIcalEvent.AddChildEvent(&childIcalEvent)
					} else {
						slog.Warn("can't cast event to child event", "where", "routes/ical.go", "eventID", undecidedIcalEvent.GetID())
						continue
					}
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
