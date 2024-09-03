package model

import (
	"github.com/uptrace/bun"
)

type Attendee struct {
	bun.BaseModel `bun:"table:attendees"`

	EventID string `bun:"event_id,notnull"` // required
	Data    string `bun:"data,notnull"`     // required

	MasterEvent *MasterEvent `bun:"rel:belongs-to,join:event_id=id"`
	ChildEvent  *ChildEvent  `bun:"rel:belongs-to,join:event_id=id"`
}
