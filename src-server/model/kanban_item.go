package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Each Kanban group has many Kanban items
type KanbanItem struct {
	bun.BaseModel `bun:"table:kanban_items"`

	ID        int64  `bun:"id,pk,autoincrement"`
	Content   string `bun:"content,notnull"`    // required
	GroupName string `bun:"group_name,notnull"` // required
	ChannelID string `bun:"channel_id,notnull"` // required

	Group *KanbanGroup `bun:"rel:belongs-to,join:group_name=name"`
	Table *KanbanTable `bun:"rel:belongs-to,join:channel_id=channel_id"`
}

func (k *KanbanItem) Upsert(ctx context.Context, db bun.IDB) error {
	if k.Content == "" {
		return fmt.Errorf("(*KanbanItem).Upsert: content is required")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(k).
		On("CONFLICT (id) DO UPDATE").
		Set("content = EXCLUDED.content").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*KanbanItem).Upsert: %w", err)
	}

	return nil
}
