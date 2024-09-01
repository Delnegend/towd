package model

import "github.com/uptrace/bun"

type SessionToken struct {
	bun.BaseModel `bun:"table:session_tokens"`

	Secret    string `bun:"secret,pk,notnull,unique"`
	UserID    string `bun:"user_id,notnull"`
	CreatedAt int64  `bun:"created_at,notnull"`
	IpAddress string `bun:"ip_address,notnull"`
	UserAgent string `bun:"user_agent"`
}
