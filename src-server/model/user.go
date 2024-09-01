package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users"`

	ID         string `bun:"id,pk,notnull,unique"`
	Username   string `bun:"username,notnull"`
	TotpSecret string `bun:"totp_secret,notnull,unique"`
}

func (u *User) Upsert(ctx context.Context, db bun.IDB) error {
	if u.ID == "" {
		return fmt.Errorf("(*User).Upsert: missing ID")
	}

	if _, err := db.
		NewInsert().
		Model(u).
		On("CONFLICT (id) DO UPDATE").
		Set("totp_secret = EXCLUDED.totp_secret").
		Set("username = EXCLUDED.username").
		Set("totp_secret = EXCLUDED.totp_secret").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*User).Upsert: %w", err)
	}

	return nil
}
