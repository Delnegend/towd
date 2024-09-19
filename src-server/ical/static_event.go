package ical

import (
	"log/slog"
	"time"
	"towd/src-server/ical/event"

	"github.com/google/uuid"
)

// Pure event information
type StaticEvent struct {
	ID          string `json:"id"`         // required
	StartDate   int64  `json:"start_date"` // required
	EndDate     int64  `json:"end_date"`   // required
	IsWholeDay  bool   `json:"is_whole_day"`
	Title       string `json:"title"` // required
	Description string `json:"description"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Organizer   string `json:"organizer"`
}

func (c *Calendar) ToStaticEvents() []StaticEvent {
	staticEvents := make([]StaticEvent, 0)

	c.IterateMasterEvents(func(id string, masterEvent *event.MasterEvent) error {
		rruleSet, err := masterEvent.GetRRuleSet()
		switch {
		case err != nil:
			slog.Warn("can't get rrule set", "where", "ical.StaticEvent.ToStaticEvents", "error", err)
		case rruleSet == nil:
			staticEvents = append(staticEvents, StaticEvent{
				ID:        masterEvent.GetID(),
				StartDate: masterEvent.GetStartDate(),
				EndDate:   masterEvent.GetEndDate(),
				IsWholeDay: func() bool {
					startDate := time.Unix(masterEvent.GetStartDate(), 0)
					return startDate.Hour() == 0 && startDate.Minute() == 0
				}(),
				Title:       masterEvent.GetSummary(),
				Description: masterEvent.GetDescription(),
				Location:    masterEvent.GetLocation(),
				URL:         masterEvent.GetURL(),
				Organizer:   masterEvent.GetOrganizer(),
			})
		case rruleSet != nil:
			// get all dates from rrule set
			allDates := func() map[int64]struct{} {
				allDates := make(map[int64]struct{})
				for _, date := range rruleSet.All() {
					allDates[date.UTC().Unix()] = struct{}{}
				}
				return allDates
			}()

			startEndDiff := masterEvent.GetEndDate() - masterEvent.GetStartDate()

			// append its child events first and remove corresponding recurrence
			// IDs from allDates since they're overwriting the master event's
			// occurrences
			masterEvent.IterateChildEvents(func(id string, childEvent *event.ChildEvent) error {
				delete(allDates, childEvent.GetRecurrenceID())
				staticEvents = append(staticEvents, StaticEvent{
					ID:        uuid.NewString(),
					StartDate: childEvent.GetRecurrenceID(),
					EndDate:   childEvent.GetRecurrenceID() + startEndDiff,
					IsWholeDay: func() bool {
						startDate := time.Unix(childEvent.GetRecurrenceID(), 0)
						return startDate.Hour() == 0 && startDate.Minute() == 0
					}(),
					Title:       childEvent.GetSummary(),
					Description: childEvent.GetDescription(),
					Location:    childEvent.GetLocation(),
					URL:         childEvent.GetURL(),
					Organizer:   childEvent.GetOrganizer(),
				})
				return nil
			})

			// create clones
			for date := range allDates {
				staticEvents = append(staticEvents, StaticEvent{
					ID:        uuid.NewString(),
					StartDate: date,
					EndDate:   date + startEndDiff,
					IsWholeDay: func() bool {
						startDate := time.Unix(date, 0)
						return startDate.Hour() == 0 && startDate.Minute() == 0
					}(),
					Title:       masterEvent.GetSummary(),
					Description: masterEvent.GetDescription(),
					Location:    masterEvent.GetLocation(),
					URL:         masterEvent.GetURL(),
					Organizer:   masterEvent.GetOrganizer(),
				})
			}
		}
		return nil
	})

	return staticEvents
}
