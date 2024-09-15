package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Session struct {
	bun.BaseModel `bun:"table:sessions"`

	Secret    string    `bun:"secret,pk"`          // required
	Type      string    `bun:"type,notnull"`       // required
	UserID    string    `bun:"user_id,notnull"`    // required
	ChannelID string    `bun:"channel_id,notnull"` // required
	CreatedAt time.Time `bun:"created_at,notnull"` // required
}
