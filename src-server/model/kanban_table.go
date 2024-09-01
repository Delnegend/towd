package model

import "github.com/uptrace/bun"

type KanbanTable struct {
	bun.BaseModel `bun:"table:kanbans"`

	Name      string `bun:"name,pk,notnull,unique"`
	ChannelID string `bun:"channel_id,notnull,unique"`

	Groups []KanbanGroup `bun:"rel:has-many,join:kanban_id=kanban_id"`
	Items  []KanbanItem  `bun:"rel:has-many,join:kanban_id=kanban_id"`
}
