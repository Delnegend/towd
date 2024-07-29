package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

func CreateSchema(db *bun.DB) error {
	if err := db.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, model := range []interface{}{
			(*Calendar)(nil),
			(*MasterEvent)(nil),
			(*ChildEvent)(nil),
			(*Attendee)(nil),
			(*RRule)(nil),
			(*User)(nil),
		} {
			if _, err := tx.
				NewCreateTable().
				Model(model).
				Exec(context.Background()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("CreateSchema: %w", err)
	}

	return nil
}
