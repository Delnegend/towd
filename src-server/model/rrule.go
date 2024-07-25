package model

import "github.com/uptrace/bun"

// This model stores the unmarshalled rrule set of a master event
type RRule struct {
	bun.BaseModel `bun:"table:rrules"`

	EventID string `bun:"event_id,notnull"`
	Date    int64  `bun:"date,notnull"`
}
