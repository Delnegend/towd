package model

import (
	"context"
	"fmt"
	"net/url"
	"towd/src-server/ical/event"

	"github.com/uptrace/bun"
)

type ChildEventIDsCtxKeyType string

const ChildEventIDCtxKey ChildEventIDsCtxKeyType = "child-event-ids"

type ChildEvent struct {
	bun.BaseModel `bun:"table:child_events"`

	// id
	// - must be the same as the master event id
	// recurrence id
	// - act as exdates (it match any of the dates in the rrule set)
	// - fill in the excluded date with another event
	ID           string `bun:"id,notnull"`
	RecurrenceID int64  `bun:"recurrence_id,notnull"`

	Summary     string `bun:"summary,notnull"`
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDate int64 `bun:"start_date,notnull"`
	EndDate   int64 `bun:"end_date,notnull"`

	CreatedAt int64 `bun:"created_at,notnull"`
	UpdatedAt int64 `bun:"updated_at,notnull"`
	Sequence  int   `bun:"sequence"`

	Event *MasterEvent `bun:"rel:belongs-to,join:id=id"`
}

var _ bun.AfterDeleteHook = (*ChildEvent)(nil)

func (c *ChildEvent) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("ChildEvent.AfterDelete: db is nil")
	}

	switch childEventID := ctx.Value(ChildEventIDCtxKey).(type) {
	case string:
		if childEventID == "" {
			return fmt.Errorf("ChildEvent.AfterDelete: childEventID is blank")
		}

		// get the going-to-be-deleted attendee ids before deleting them
		deletedAttendeeIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*Attendee)(nil)).
			Column("event_id").
			Where("event_id = ?", childEventID).
			Scan(ctx, &deletedAttendeeIDs); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't get attendee ids: %w", err)
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id = ?", childEventID).
			Exec(ctx); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't delete attendees: %w", err)
		}
	case []string:
		if len(childEventID) == 0 {
			return fmt.Errorf("ChildEvent.AfterDelete: childEventID is empty")
		}

		// get the going-to-be-deleted attendee ids before deleting them
		attendeeIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*Attendee)(nil)).
			Column("event_id").
			Where("event_id IN (?)", bun.In(childEventID)).
			Scan(ctx, &attendeeIDs); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't get attendee ids: %w", err)
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id IN (?)", bun.In(childEventID)).
			Exec(context.WithValue(ctx, AttendeeIDCtxKey, attendeeIDs)); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't delete attendees: %w", err)
		}
	case nil:
		return fmt.Errorf("ChildEvent.AfterDelete: child event ids is nil")
	default:
		return fmt.Errorf("ChildEvent.AfterDelete: wrong childEventID type | type=%T", childEventID)
	}

	return nil
}

func (c *ChildEvent) FromIcal(
	ctx context.Context,
	db bun.IDB,
	childEvent *event.ChildEvent,
) error {
	if db == nil {
		return fmt.Errorf("FromIcalChildEventToDB: db is nil")
	}

	c.ID = childEvent.GetID()
	c.RecurrenceID = childEvent.GetRecurrenceID()

	c.Summary = childEvent.GetSummary()
	c.Description = childEvent.GetDescription()
	c.Location = childEvent.GetLocation()
	c.URL = childEvent.GetURL()
	c.Organizer = childEvent.GetOrganizer()

	c.StartDate = childEvent.GetStartDate()
	c.EndDate = childEvent.GetEndDate()

	c.CreatedAt = childEvent.GetCreatedAt()
	c.UpdatedAt = childEvent.GetUpdatedAt()
	c.Sequence = childEvent.GetSequence()

	return nil
}

func (e *ChildEvent) Upsert(ctx context.Context, db bun.IDB) error {
	// basic field validation
	switch {
	case e.Summary == "":
		return fmt.Errorf("ChildEvent.Upsert: summary is required")
	case e.RecurrenceID == 0:
		return fmt.Errorf("ChildEvent.Upsert: recurrence id is required")
	case e.CreatedAt == 0:
		return fmt.Errorf("ChildEvent.Upsert: created at is required")
	case e.UpdatedAt != 0 && e.UpdatedAt < e.CreatedAt:
		return fmt.Errorf("ChildEvent.Upsert: updated at must be after created at")
	case e.StartDate == 0:
		return fmt.Errorf("ChildEvent.Upsert: start date is required")
	case e.EndDate == 0:
		return fmt.Errorf("ChildEvent.Upsert: end date is required")
	case e.StartDate > e.EndDate:
		return fmt.Errorf("ChildEvent.Upsert: start date must be before end date")
	}
	if e.URL != "" {
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("ChildEvent.Upsert: %w", err)
		}
	}

	// check if master event exists
	exist, err := db.NewSelect().
		Model(&MasterEvent{}).
		Where("id = ?", e.ID).
		Exists(context.Background())
	if err != nil {
		return fmt.Errorf("ChildEvent.Upsert: %w", err)
	}
	if !exist {
		return fmt.Errorf("ChildEvent.Upsert: master event id not found")
	}

	// check if from a read-only calendar
	masterEventModal := new(MasterEvent)
	if err := db.NewSelect().
		Model(masterEventModal).
		Where("id = ?", e.ID).
		Scan(ctx, masterEventModal); err != nil {
		return fmt.Errorf("ChildEvent.Upsert: can't get master event: %w", err)
	}
	calendarModal := new(Calendar)
	if err := db.NewSelect().
		Model(calendarModal).
		Where("id = ?", masterEventModal.CalendarID).
		Scan(ctx, calendarModal); err != nil {
		return fmt.Errorf("ChildEvent.Upsert: can't get calendar: %w", err)
	}
	if calendarModal.Url != "" {
		return fmt.Errorf("ChildEvent.Upsert: this event is from a read-only calendar")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(e).
		Exec(ctx); err != nil {
		return fmt.Errorf("ChildEvent.Upsert: %w", err)
	}

	return nil
}
