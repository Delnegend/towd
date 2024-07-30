package model

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"towd/src-server/ical/event"

	"github.com/uptrace/bun"
	"github.com/xyedo/rrule"
)

type MasterEventIDCtxKeyType string

const MasterEventIDCtxKey MasterEventIDCtxKeyType = "master-event-ids"

type MasterEvent struct {
	bun.BaseModel `bun:"table:master_events"`

	ID          string `bun:"id,pk,notnull"`
	CalendarID  string `bun:"calendar_id,notnull"`
	Summary     string `bun:"summary,notnull"`
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDate int64 `bun:"start_date,notnull"`
	EndDate   int64 `bun:"end_date,notnull"`

	CreatedAt int64  `bun:"created_at,notnull"`
	UpdatedAt int64  `bun:"updated_at"`
	Sequence  int    `bun:"sequence"`
	RRule     string `bun:"rrule"`
	RDate     string `bun:"rdate"`
	ExDate    string `bun:"exdate"`

	Calendar *Calendar `bun:"rel:belongs-to,join:calendar_id=id"`
}

var _ bun.AfterDeleteHook = (*MasterEvent)(nil)

// Cleanup child events, attendees, and parsed rrules
func (m *MasterEvent) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("MasterEvent.AfterDelete: db is nil")
	}

	switch masterEventID := ctx.Value(MasterEventIDCtxKey).(type) {
	case string:
		if masterEventID == "" {
			return fmt.Errorf("MasterEvent.AfterDelete: deletedMasterEventID is blank")
		}

		// rm related rrule
		if _, err := query.DB().NewDelete().
			Model((*RRule)(nil)).
			Where("event_id = ?", masterEventID).
			Exec(ctx); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete rrule: %w", err)
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id = ?", masterEventID).
			Exec(ctx); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete attendees: %w", err)
		}

		// rm related child events
		if _, err := query.DB().NewDelete().
			Model((*ChildEvent)(nil)).
			Where("id = ?", masterEventID).
			Exec(context.WithValue(ctx, ChildEventIDCtxKey, func() []string {
				childEventIDs := []string{}
				if err := query.DB().NewSelect().
					Model((*ChildEvent)(nil)).
					Column("id").
					Where("id = ?", masterEventID).
					Scan(ctx, &childEventIDs); err != nil {
					slog.Warn("MasterEvent.AfterDelete: can't get child event ids to inject to context", "error", err)
					return []string{}
				}
				return childEventIDs
			}())); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete child events: %w", err)
		}
	case []string:
		if len(masterEventID) == 0 {
			return fmt.Errorf("MasterEvent.AfterDelete: deletedMasterEventID is empty")
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id IN (?)", bun.In(masterEventID)).
			Exec(ctx); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete attendees: %w", err)
		}

		// rm related rrule
		if _, err := query.DB().NewDelete().
			Model((*RRule)(nil)).
			Where("event_id IN (?)", bun.In(masterEventID)).
			Exec(ctx); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete rrule: %w", err)
		}

		// rm related child events
		if _, err := query.DB().NewDelete().
			Model((*ChildEvent)(nil)).
			Where("id IN (?)", bun.In(masterEventID)).
			Exec(context.WithValue(ctx, ChildEventIDCtxKey, func() []string {
				childEventIDs := []string{}
				if err := query.DB().NewSelect().
					Model((*ChildEvent)(nil)).
					Column("id").
					Where("id IN (?)", bun.In(masterEventID)).
					Scan(ctx, &childEventIDs); err != nil {
					slog.Warn("MasterEvent.AfterDelete: can't get child event ids to inject to context", "error", err)
					return []string{}
				}
				return childEventIDs

			})); err != nil {
			return fmt.Errorf("MasterEvent.AfterDelete: can't delete child events: %w", err)
		}
	case nil:
		return fmt.Errorf("MasterEvent.AfterDelete: master event id is nil")
	default:
		return fmt.Errorf("MasterEvent.AfterDelete: wrong master event id type | type=%T", masterEventID)
	}

	return nil
}

// Create a new MasterEvent from an ical master event
func (m *MasterEvent) FromIcal(
	ctx context.Context,
	db bun.IDB,
	masterEvent *event.MasterEvent,
	calendarID string,
) error {
	if db == nil {
		return fmt.Errorf("FromIcal: db is nil")
	}

	m.ID = masterEvent.GetID()
	m.CalendarID = calendarID
	m.Summary = masterEvent.GetSummary()
	m.Description = masterEvent.GetDescription()
	m.Location = masterEvent.GetLocation()
	m.URL = masterEvent.GetURL()
	m.Organizer = masterEvent.GetOrganizer()

	m.StartDate = masterEvent.GetStartDate()
	m.EndDate = masterEvent.GetEndDate()

	m.CreatedAt = masterEvent.GetCreatedAt()
	m.UpdatedAt = masterEvent.GetUpdatedAt()
	m.Sequence = masterEvent.GetSequence()

	if masterEvent.GetRRuleSet() != nil {
		m.RRule = masterEvent.GetRRuleSet().String()
		var rdates []string
		masterEvent.IterateRDates(func(unixTime int64) {
			rdates = append(rdates, fmt.Sprintf("%d", unixTime))
		})
		m.RDate = strings.Join(rdates, ",")
		var exdates []string
		masterEvent.IterateExDates(func(unixTime int64) {
			exdates = append(exdates, fmt.Sprintf("%d", unixTime))
		})
		m.ExDate = strings.Join(exdates, ",")
	}

	return nil
}

// Upsert the master event to the database
func (e *MasterEvent) Upsert(ctx context.Context, db bun.IDB) error {
	// validate
	switch {
	case e.Summary == "":
		return fmt.Errorf("MasterEvent.Upsert: summary is required")
	case e.CalendarID == "":
		return fmt.Errorf("MasterEvent.Upsert: calendar id is required")
	case e.CreatedAt == 0:
		return fmt.Errorf("MasterEvent.Upsert: created at is required")
	case e.StartDate == 0:
		return fmt.Errorf("MasterEvent.Upsert: start date is required")
	case e.EndDate == 0:
		return fmt.Errorf("MasterEvent.Upsert: end date is required")
	case e.StartDate > e.EndDate:
		return fmt.Errorf("MasterEvent.Upsert: start date must be before end date")
	case e.URL != "":
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("MasterEvent.Upsert: url is invalid: %w", err)
		}
	case e.RRule == "" && (e.RDate != "" || e.ExDate != ""):
		return fmt.Errorf("MasterEvent.Upsert: rdate/exdate only works with rrule")
	}
	if e.URL != "" {
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("MasterEvent.Upsert: url is invalid: %w", err)
		}
	}
	var rruleSet *rrule.Set
	if e.RRule != "" {
		var err error
		if rruleSet, err = rrule.StrToRRuleSet(e.RRule); err != nil {
			return fmt.Errorf("MasterEvent.Upsert: invalid rrule: %w", err)
		}
	}

	// check if calendar exists
	calendarExist, err := db.NewSelect().
		Model(&Calendar{}).
		Where("id = ?", e.CalendarID).
		Exists(context.Background())
	if err != nil {
		return err
	}
	if !calendarExist {
		return fmt.Errorf("MasterEvent.Upsert: calendar id not found")
	}

	// check if calendar is read-only
	calendarModal := new(Calendar)
	if err := db.NewSelect().
		Model(calendarModal).
		Where("id = ?", e.CalendarID).
		Scan(ctx, calendarModal); err != nil {
		return fmt.Errorf("MasterEvent.Upsert: can't get calendar: %w", err)
	}
	if calendarModal.Url != "" {
		return fmt.Errorf("MasterEvent.Upsert: this event is from a read-only calendar")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(e).
		On("CONFLICT (id) DO UPDATE").
		Set("id = EXCLUDED.id").
		Set("calendar_id = EXCLUDED.calendar_id").
		Set("summary = EXCLUDED.summary").
		Set("description = EXCLUDED.description").
		Set("location = EXCLUDED.location").
		Set("url = EXCLUDED.url").
		Set("organizer = EXCLUDED.organizer").
		Set("start_date = EXCLUDED.start_date").
		Set("end_date = EXCLUDED.end_date").
		Set("created_at = EXCLUDED.created_at").
		Set("updated_at = EXCLUDED.updated_at").
		Set("sequence = EXCLUDED.sequence").
		Set("rrule = EXCLUDED.rrule").
		Set("rdate = EXCLUDED.rdate").
		Set("exdate = EXCLUDED.exdate").
		Exec(ctx); err != nil {
		return fmt.Errorf("MasterEvent.Upsert: %w", err)
	}

	// remove all parsed rrules
	if _, err := db.NewDelete().
		Model((*RRule)(nil)).
		Where("event_id = ?", e.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("MasterEvent.Upsert: %w", err)
	}

	if rruleSet != nil {
		parsedUnixFromRRule := make(map[int64]struct{})
		for _, date := range rruleSet.All() {
			parsedUnixFromRRule[date.Unix()] = struct{}{}
		}

		// insert new parsed rrules
		for date := range parsedUnixFromRRule {
			rruleModel := RRule{
				EventID: e.ID,
				Date:    date,
			}
			if _, err := db.NewInsert().
				Model(&rruleModel).
				Exec(ctx); err != nil {
				return fmt.Errorf("MasterEvent.Upsert: %w", err)
			}
		}

		// remove child events that doesn't include parsed rrule dates
		if _, err := db.NewDelete().
			Model((*ChildEvent)(nil)).
			Where("id = ?", e.ID).
			Where("data NOT IN (?)", bun.In(parsedUnixFromRRule)).
			Exec(context.WithValue(
				ctx,
				ChildEventIDCtxKey,
				func() []string {
					childEventModels := make([]ChildEvent, 0)
					if err := db.NewSelect().
						Model(&childEventModels).
						Where("id = ?", e.ID).
						Scan(ctx, &childEventModels); err != nil {
						return nil
					}
					IDs := make([]string, 0)
					for _, childEventModel := range childEventModels {
						IDs = append(IDs, childEventModel.ID)
					}
					return IDs
				}()),
			); err != nil {
			return fmt.Errorf("MasterEvent.Upsert: %w", err)
		}
	} else {
		// remove all child events since there's no recurrence rule
		if _, err := db.NewDelete().
			Model((*ChildEvent)(nil)).
			Where("id = ?", e.ID).
			Exec(context.WithValue(
				ctx,
				ChildEventIDCtxKey,
				func() []string {
					childEventModels := make([]ChildEvent, 0)
					if err := db.NewSelect().
						Model(&childEventModels).
						Where("id = ?", e.ID).
						Scan(ctx, &childEventModels); err != nil {
						return nil
					}
					IDs := make([]string, 0)
					for _, childEventModel := range childEventModels {
						IDs = append(IDs, childEventModel.ID)
					}
					return IDs
				}()),
			); err != nil {
			return fmt.Errorf("MasterEvent.Upsert: %w", err)
		}
	}

	return nil
}
