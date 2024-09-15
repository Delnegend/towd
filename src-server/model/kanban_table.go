package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type TableIDCtxKeyType string

const TableIDCtxKey TableIDCtxKeyType = "table-id"

// Each channel has one Kanban table
type KanbanTable struct {
	bun.BaseModel `bun:"table:kanbans"`

	Name      string `bun:"name,pk"`                   // required
	ChannelID string `bun:"channel_id,notnull,unique"` // required

	Groups []*KanbanGroup `bun:"rel:has-many,join:channel_id=channel_id"`
	Items  []*KanbanItem  `bun:"rel:has-many,join:channel_id=channel_id"`
}

var _ bun.AfterDeleteHook = (*KanbanTable)(nil)

func (k *KanbanTable) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("(*KanbanTable).AfterDelete: db is nil")
	}

	switch tableID := ctx.Value(TableIDCtxKey).(type) {
	case string:
		if tableID == "" {
			return fmt.Errorf("(*KanbanTable).AfterDelete: table id is blank")
		}

		// rm related kanban groups
		if _, err := query.DB().NewDelete().
			Model((*KanbanGroup)(nil)).
			Where("channel_id = ?", tableID).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*KanbanTable).AfterDelete: %w", err)
		}

		// rm related kanban items
		if _, err := query.DB().NewDelete().
			Model((*KanbanItem)(nil)).
			Where("channel_id = ?", tableID).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*KanbanTable).AfterDelete: %w", err)
		}
	case []string:
		if len(tableID) == 0 {
			return fmt.Errorf("(*KanbanTable).AfterDelete: table id is empty")
		}

		// rm related kanban groups
		if _, err := query.DB().NewDelete().
			Model((*KanbanGroup)(nil)).
			Where("channel_id IN (?)", bun.In(tableID)).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*KanbanTable).AfterDelete: %w", err)
		}

		// rm related kanban items
		if _, err := query.DB().NewDelete().
			Model((*KanbanItem)(nil)).
			Where("channel_id IN (?)", bun.In(tableID)).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*KanbanTable).AfterDelete: %w", err)
		}
	case nil:
		return fmt.Errorf("(*KanbanTable).AfterDelete: table id is nil")
	default:
		return fmt.Errorf("(*KanbanTable).AfterDelete: wrong table id type | type=%T", tableID)
	}

	return nil
}

func (k *KanbanTable) Upsert(ctx context.Context, db bun.IDB) error {
	if k.Name == "" {
		return fmt.Errorf("(*KanbanTable).Upsert: name is required")
	}
	if k.ChannelID == "" {
		return fmt.Errorf("(*KanbanTable).Upsert: channel id is required")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(k).
		On("CONFLICT (name) DO UPDATE").
		Set("name = EXCLUDED.name").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*KanbanTable).Upsert: %w", err)
	}

	return nil
}
