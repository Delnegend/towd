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
	TotpSecret string `json:"totp_secret"`
}

func (u *User) Upsert(ctx context.Context, db bun.IDB) error {
	if u.ID == "" {
		return fmt.Errorf("user id is empty")
	}

	_, err := db.
		NewInsert().
		Model(u).
		On("CONFLICT (id) DO UPDATE").
		Set("totp_secret = EXCLUDED.totp_secret").
		Exec(ctx)

	return err
}
