package metric

import (
	"context"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"
)

func database(as *utils.AppState) (time.Duration, error) {
	start := time.Now()
	if _, err := as.BunDB.NewSelect().
		Model((*model.Event)(nil)).
		Where("channel_id = ?", "").
		Exists(context.Background()); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
