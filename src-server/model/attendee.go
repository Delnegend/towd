package model

import "github.com/uptrace/bun"

type AttendeeIDCtxKeyType string

const AttendeeIDCtxKey AttendeeIDCtxKeyType = "attendee-ids"

type Attendee struct {
	bun.BaseModel `bun:"table:attendees"`

	EventID string `bun:"event_id,notnull"`
	Data    string `bun:"data,notnull"`
}
