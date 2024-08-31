package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type DeletedCalendarIDsCtxKeyType string

const DeletedCalendarIDsCtxKey DeletedCalendarIDsCtxKeyType = "calendar-id"

type Calendar struct {
	bun.BaseModel `bun:"table:calendars"`

	ID          string `bun:"id,pk,notnull,unique"`
	ProdID      string `bun:"calendar_id"`
	Name        string `bun:"name"`
	Description string `bun:"description"`
	Url         string `bun:"url,unique"`
	Hash        string `bun:"hash,unique"`

	ChannelID    string        `bun:"channel_id"`
	MasterEvents []MasterEvent `bun:"rel:has-many,join:id=calendar_id"`
}

var _ bun.AfterDeleteHook = (*Calendar)(nil)

func (c *Calendar) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("Calendar.AfterDelete: db is nil")
	}

	switch deletedCalendarID := ctx.Value(DeletedCalendarIDsCtxKey).(type) {
	case string:
		if deletedCalendarID == "" {
			return fmt.Errorf("Calendar.AfterDelete: deletedCalendarID is blank")
		}

		// get the going-to-be-deleted master event ids before deleting them
		deletedMasterEventIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*MasterEvent)(nil)).
			Column("id").
			Where("calendar_id = ?", deletedCalendarID).
			Scan(ctx, &deletedMasterEventIDs); err != nil {
			return fmt.Errorf("Calendar.AfterDelete: can't get deleted master event ids: %w", err)
		}

		// rm master events of the calendar
		if _, err := query.DB().NewDelete().
			Model((*MasterEvent)(nil)).
			Where("calendar_id = ?", deletedCalendarID).
			Exec(context.WithValue(ctx, MasterEventIDCtxKey, deletedMasterEventIDs)); err != nil {
			return fmt.Errorf("Calendar.AfterDelete: can't delete master events: %w", err)
		}
	case []string:
		if len(deletedCalendarID) == 0 {
			return fmt.Errorf("Calendar.AfterDelete: deletedCalendarID is empty")
		}

		// get the going-to-be-deleted master event ids before deleting them
		deletedMasterEventIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*MasterEvent)(nil)).
			Column("id").
			Where("calendar_id IN (?)", bun.In(deletedCalendarID)).
			Scan(ctx, &deletedMasterEventIDs); err != nil {
			return fmt.Errorf("Calendar.AfterDelete: can't get deleted master event ids: %w", err)
		}

		// rm master events of the calendar
		if _, err := query.DB().NewDelete().
			Model((*MasterEvent)(nil)).
			Where("calendar_id IN (?)", bun.In(deletedCalendarID)).
			Exec(context.WithValue(ctx, MasterEventIDCtxKey, deletedMasterEventIDs)); err != nil {
			return fmt.Errorf("Calendar.AfterDelete: can't delete master events: %w", err)
		}
	case nil:
		return fmt.Errorf("Calendar.AfterDelete: calendar id is nil")
	default:
		return fmt.Errorf("Calendar.AfterDelete: wrong deletedCalendarID type | type=%T", deletedCalendarID)
	}

	return nil
}

func (c *Calendar) Upsert(ctx context.Context, db bun.IDB) error {
	if db == nil {
		return fmt.Errorf("Calendar.Upsert: db is nil")
	}

	// vaidate
	switch {
	case c.ID == "":
		return fmt.Errorf("Calendar.Upsert: calendar id is blank")
	case c.Name == "":
		return fmt.Errorf("Calendar.Upsert: calendar name is blank")
	}

	// upsert
	if _, err := db.NewInsert().
		Model(c).
		On("CONFLICT (id) DO UPDATE").
		Set("prod_id = EXCLUDED.prod_id").
		Set("name = EXCLUDED.name").
		Set("description = EXCLUDED.description").
		Set("url = EXCLUDED.url").
		Exec(ctx); err != nil {
		return fmt.Errorf("Calendar.Upsert: can't upsert calendar: %w", err)
	}

	return nil
}
