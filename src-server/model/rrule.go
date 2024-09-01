package model

import "github.com/uptrace/bun"

// Parsed dates from RRule sets from master events
type RRule struct {
	bun.BaseModel `bun:"table:rrules"`

	EventID string `bun:"event_id,notnull"`
	Date    int64  `bun:"date,notnull"`
}
