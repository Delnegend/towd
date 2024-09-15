package model

import (
	"github.com/uptrace/bun"
)

type Attendee struct {
	bun.BaseModel `bun:"table:attendees"`

	EventID string `bun:"event_id,notnull"` // required
	Data    string `bun:"data,notnull"`     // required

	Event *Event `bun:"rel:belongs-to,join:event_id=id"`
}
