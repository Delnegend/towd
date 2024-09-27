package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type KanbanBoardChannelIDCtxKeyType string

const KanbanBoardChannelIDCtxKey KanbanBoardChannelIDCtxKeyType = "table-id"

// Each channel has one Kanban table
type KanbanTable struct {
	bun.BaseModel `bun:"table:kanbans"`

	Name      string `bun:"name,pk"`                   // required
	ChannelID string `bun:"channel_id,notnull,unique"` // required

	Groups []*KanbanGroup `bun:"rel:has-many,join:channel_id=channel_id"`
	Items  []*KanbanItem  `bun:"rel:has-many,join:channel_id=channel_id"`
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
