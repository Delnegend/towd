package model

import "github.com/uptrace/bun"

type Calendar struct {
	bun.BaseModel `bun:"table:calendars"`

	ChannelID string `bun:"channel_id,pk"` // required
	Name      string `bun:"name,notnull"`  // required

	Events []*Event `bun:"rel:has-many,join:channel_id=channel_id"`
}
