package model

import "github.com/uptrace/bun"

type KanbanGroup struct {
	bun.BaseModel `bun:"table:kanban_groups"`

	Name     string `bun:"name,pk,notnull"`
	KanbanID string `bun:"kanban_id,notnull"`

	Items []KanbanItem `bun:"rel:has-many,join:kanban_id=kanban_id"`
	Table *KanbanTable `bun:"rel:belongs-to,join:kanban_id=kanban_id"`
}
