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
			(*Attendee)(nil),
			(*Calendar)(nil),
			(*Event)(nil),
			(*ExternalCalendar)(nil),
			(*KanbanGroup)(nil),
			(*KanbanItem)(nil),
			(*KanbanTable)(nil),
			(*Session)(nil),
		} {
			if _, err := tx.
				NewCreateTable().
				Model(model).
				IfNotExists().
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
