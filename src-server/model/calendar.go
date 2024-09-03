package model

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

type DeletedCalendarIDsCtxKeyType string

const DeletedCalendarIDsCtxKey DeletedCalendarIDsCtxKeyType = "calendar-id"

type Calendar struct {
	bun.BaseModel `bun:"table:calendars"`

	ID          string `bun:"id,pk"` // required
	ProdID      string `bun:"prod_id"`
	Name        string `bun:"name,notnull"` // required
	Description string `bun:"description"`
	Url         string `bun:"url,unique"`
	Hash        string `bun:"hash,unique"`

	ChannelID    string        `bun:"channel_id"`
	MasterEvents []MasterEvent `bun:"rel:has-many,join:id=calendar_id"`
}

var _ bun.AfterDeleteHook = (*Calendar)(nil)

func (c *Calendar) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("(*Calendar).AfterDelete: db is nil")
	}

	deletedCalendarIDs := make([]string, 0)
	switch deletedCalendarID := ctx.Value(DeletedCalendarIDsCtxKey).(type) {
	case string:
		if deletedCalendarID == "" {
			return fmt.Errorf("(*Calendar).AfterDelete: deletedCalendarID is blank")
		}

		deletedCalendarIDs = append(deletedCalendarIDs, deletedCalendarID)
	case []string:
		if len(deletedCalendarID) == 0 {
			return nil
		}
		deletedCalendarIDs = append(deletedCalendarIDs, deletedCalendarID...)
	case nil:
		return fmt.Errorf("(*Calendar).AfterDelete: calendar id is nil")
	default:
		return fmt.Errorf("(*Calendar).AfterDelete: wrong deletedCalendarID type | type=%T", deletedCalendarID)
	}

	// delete all related master events
	if _, err := query.DB().NewDelete().
		Model((*MasterEvent)(nil)).
		Where("calendar_id IN (?)", bun.In(deletedCalendarIDs)).
		Exec(context.WithValue(ctx, MasterEventIDCtxKey, func() []string {
			masterEventModels := make([]MasterEvent, 0)
			if err := query.DB().NewSelect().
				Model(&masterEventModels).
				Column("id").
				Where("calendar_id IN (?)", bun.In(deletedCalendarIDs)).
				Scan(ctx); err != nil {
				slog.Warn("can't get deleted master event ids", "error", err)
				return []string{}
			}
			masterEventIDs := make([]string, 0)
			for _, masterEventModel := range masterEventModels {
				masterEventIDs = append(masterEventIDs, masterEventModel.ID)
			}
			return masterEventIDs
		}())); err != nil {
		return fmt.Errorf("(*Calendar).AfterDelete: can't delete master events: %w", err)
	}

	return nil
}

func (c *Calendar) Upsert(ctx context.Context, db bun.IDB) error {
	if db == nil {
		return fmt.Errorf("(*Calendar).Upsert: db is nil")
	}

	// vaidate
	switch {
	case c.ID == "":
		return fmt.Errorf("(*Calendar).Upsert: calendar id is blank")
	case c.Name == "":
		return fmt.Errorf("(*Calendar).Upsert: calendar name is blank")
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
		return fmt.Errorf("(*Calendar).Upsert: can't upsert calendar: %w", err)
	}

	return nil
}
