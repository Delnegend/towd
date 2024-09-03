package model

import "github.com/uptrace/bun"

// Parsed dates from RRule sets from master events
type RRule struct {
	bun.BaseModel `bun:"table:rrules"`

	EventID  string `bun:"event_id,notnull"`  // required
	UnixDate int64  `bun:"unix_date,notnull"` // required

	MasterEvent *MasterEvent `bun:"rel:belongs-to,join:event_id=id"`
}
