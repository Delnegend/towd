package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type DeletedCalendarIDsCtxKeyType string

const DeletedCalendarIDsCtxKey DeletedCalendarIDsCtxKeyType = "calendar-id"

type ExternalCalendar struct {
	bun.BaseModel `bun:"table:external_calendars"`

	ID          string `bun:"id,pk"` // required
	ProdID      string `bun:"prod_id"`
	Name        string `bun:"name,notnull"` // required
	Description string `bun:"description"`
	Url         string `bun:"url,unique"`
	Hash        string `bun:"hash,unique"`
	ChannelID   string `bun:"channel_id"`

	Events []*Event `bun:"rel:has-many,join:id=calendar_id"`
}

func (c *ExternalCalendar) Upsert(ctx context.Context, db bun.IDB) error {
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
		Set("hash = EXCLUDED.hash").
		Set("channel_id = EXCLUDED.channel_id").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*Calendar).Upsert: can't upsert calendar: %w", err)
	}

	return nil
}
