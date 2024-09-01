package model

import "github.com/uptrace/bun"

type KanbanItem struct {
	bun.BaseModel `bun:"table:kanban_items"`

	KanbanID string `bun:"kanban_id,pk,notnull"`
	Content  string `bun:"content,notnull"`

	Group *KanbanGroup `bun:"rel:belongs-to,join:kanban_id=kanban_id"`
	Table *KanbanTable `bun:"rel:belongs-to,join:kanban_id=kanban_id"`
}
