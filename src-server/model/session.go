package model

import (
	"github.com/uptrace/bun"
)

type SessionModelPurposeType string

const (
	// for user to use to login
	SESSION_MODEL_PURPOSE_TEMP = SessionModelPurposeType("temp")
	// for the web client to keep the session
	SESSION_MODEL_PURPOSE_SESSION = SessionModelPurposeType("session")
)

type Session struct {
	bun.BaseModel `bun:"table:sessions"`

	Secret           string                  `bun:"secret,pk"`                    // required
	Purpose          SessionModelPurposeType `bun:"purpose,notnull,type:varchar"` // required
	UserID           string                  `bun:"user_id,notnull"`              // required
	ChannelID        string                  `bun:"channel_id,notnull"`           // required
	CreatedAtUnixUTC int64                   `bun:"created_at_unix_utc,notnull"`  // required
}
