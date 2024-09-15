package model_test

import (
	"context"
	"database/sql"
	"testing"
	"towd/src-server/model"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestAttendee(t *testing.T) {
	// init db
	db, err := sql.Open(sqliteshim.ShimName, ":memory:")
	if err != nil {
		t.Error(err)
	}
	bundb := bun.NewDB(db, sqlitedialect.New())

	// init tables
	for _, model := range []interface{}{
		(*model.Calendar)(nil),
		(*model.Attendee)(nil),
		(*model.Event)(nil),
	} {
		if _, err := bundb.NewCreateTable().Model(model).IfNotExists().Exec(context.Background()); err != nil {
			t.Error(err)
		}
	}

	// create models
	calendarModel := model.Calendar{
		ID:     uuid.NewString(),
		Name:   "calendar name test",
		ProdID: uuid.NewString(),
	}
	eventModel := model.Event{
		ID:         uuid.NewString(),
		CalendarID: calendarModel.ID,
		Summary:    "test",
		StartDate:  1,
		EndDate:    1,
		ChannelID:  "test",
	}
	attendeeModel := model.Attendee{
		EventID: eventModel.ID,
		Data:    "test",
	}

	// insert models
	if err := calendarModel.Upsert(context.Background(), bundb); err != nil {
		t.Error(err)
	}
	if err := eventModel.Upsert(context.Background(), bundb); err != nil {
		t.Error(err)
	}
	if _, err := bundb.NewInsert().
		Model(&attendeeModel).
		Exec(context.Background()); err != nil {
		t.Error(err)
	}

	// case: attendee data exists
	func() {
		masterEventModelTest := new(model.Event)
		if err := bundb.NewSelect().
			Model(masterEventModelTest).
			Relation("Attendees").
			Scan(context.Background()); err != nil {
			t.Error(err)
		}
		if masterEventModelTest.Attendees[0].Data != attendeeModel.Data {
			t.Error("attendee data not found")
		}
	}()

	// case: delete event and attendee data gone
	func() {
		if _, err := bundb.NewDelete().
			Model((*model.Event)(nil)).
			Where("id = ?", eventModel.ID).
			Exec(context.WithValue(context.Background(), model.EventIDCtxKey, eventModel.ID)); err != nil {
			t.Error(err)
		}
		count, err := bundb.NewSelect().
			Model((*model.Attendee)(nil)).
			Where("event_id = ?", eventModel.ID).Count(context.Background())
		if err != nil {
			t.Error(err)
		}
		if count != 0 {
			t.Error("attendee data should not exist", count)
		}
	}()
}
