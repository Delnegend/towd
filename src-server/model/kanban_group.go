package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type GroupIDCtxKeyType string

const GroupIDCtxKey GroupIDCtxKeyType = "group-name"

// Each Kanban table has many Kanban groups
type KanbanGroup struct {
	bun.BaseModel `bun:"table:kanban_groups"`

	Name      string `bun:"name,pk"`            // required
	ChannelID string `bun:"channel_id,notnull"` // required

	Items []*KanbanItem `bun:"rel:has-many,join:name=group_name"`
	Table *KanbanTable  `bun:"rel:belongs-to,join:channel_id=channel_id"`
}

func (k *KanbanGroup) Upsert(ctx context.Context, db bun.IDB) error {
	if k.Name == "" {
		return fmt.Errorf("(*KanbanGroup).Upsert: name is required")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(k).
		On("CONFLICT (channel_id, name) DO UPDATE").
		Set("name = EXCLUDED.name").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*KanbanGroup).Upsert: %w", err)
	}

	return nil
}
