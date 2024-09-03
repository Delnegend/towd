package model

import "github.com/uptrace/bun"

type SessionToken struct {
	bun.BaseModel `bun:"table:session_tokens"`

	Secret        string `bun:"secret,pk"`          // required
	UserID        string `bun:"user_id,notnull"`    // required
	CreatedAtUnix int64  `bun:"created_at,notnull"` // required
	IpAddress     string `bun:"ip_address,notnull"` // required
	UserAgent     string `bun:"user_agent"`
}
