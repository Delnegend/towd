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
		(*model.MasterEvent)(nil),
		(*model.ChildEvent)(nil),
		(*model.Attendee)(nil),
		(*model.RRule)(nil),
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
	masterEventModel := model.MasterEvent{
		ID:         uuid.NewString(),
		CalendarID: calendarModel.ID,
		Summary:    "test",
		StartDate:  1,
		EndDate:    1,
		CreatedAt:  1,
		ChannelID:  "test",
	}
	attendeeModel := model.Attendee{
		EventID: masterEventModel.ID,
		Data:    "test",
	}

	// insert models
	if err := calendarModel.Upsert(context.Background(), bundb); err != nil {
		t.Error(err)
	}
	if err := masterEventModel.Upsert(context.Background(), bundb); err != nil {
		t.Error(err)
	}
	if _, err := bundb.NewInsert().
		Model(&attendeeModel).
		Exec(context.Background()); err != nil {
		t.Error(err)
	}

	// case: attendee data exists
	func() {
		masterEventModelTest := new(model.MasterEvent)
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

	// case: delete master event and attendee data gone
	func() {
		if _, err := bundb.NewDelete().
			Model((*model.MasterEvent)(nil)).
			Where("id = ?", masterEventModel.ID).
			Exec(context.WithValue(context.Background(), model.MasterEventIDCtxKey, masterEventModel.ID)); err != nil {
			t.Error(err)
		}
		count, err := bundb.NewSelect().
			Model((*model.Attendee)(nil)).
			Where("event_id = ?", masterEventModel.ID).Count(context.Background())
		if err != nil {
			t.Error(err)
		}
		if count != 0 {
			t.Error("attendee data should not exist", count)
		}
	}()

	// case: delete master event, child event and its attendee gone
	func() {
		// re-insert back the master event model
		if err := masterEventModel.Upsert(context.Background(), bundb); err != nil {
			t.Error(err)
		}

		// create & insert new child event and its attendee models
		childEventModel := model.ChildEvent{
			ID:            uuid.NewString(),
			MasterEventID: masterEventModel.ID,
			RecurrenceID:  1,
			Summary:       "test",
			StartDate:     1,
			EndDate:       1,
			CreatedAt:     1,
			CalendarID:    calendarModel.ID,
			ChannelID:     "test",
		}
		if err := childEventModel.Upsert(context.Background(), bundb); err != nil {
			t.Error(err)
		}
		attendeeModel := model.Attendee{
			EventID: childEventModel.ID,
			Data:    "test",
		}
		if _, err := bundb.NewInsert().
			Model(&attendeeModel).
			Exec(context.Background()); err != nil {
			t.Error(err)
		}

		// double-check for the child event and its attendee model
		if exists, err := bundb.NewSelect().
			Model((*model.ChildEvent)(nil)).
			Where("id = ?", childEventModel.ID).
			Exists(context.Background()); err != nil {
			t.Error(err)
		} else if !exists {
			t.Error("child event not found")
		}
		if exists, err := bundb.NewSelect().
			Model((*model.Attendee)(nil)).
			Where("event_id = ?", childEventModel.ID).
			Exists(context.Background()); err != nil {
			t.Error(err)
		} else if !exists {
			t.Error("attendee not found")
		}

		// deleting the master event model
		if _, err := bundb.NewDelete().
			Model((*model.MasterEvent)(nil)).
			Where("id = ?", masterEventModel.ID).
			Exec(context.WithValue(context.Background(), model.MasterEventIDCtxKey, masterEventModel.ID)); err != nil {
			t.Error(err)
		}

		// master event model should not exists
		if exists, err := bundb.NewSelect().
			Model((*model.MasterEvent)(nil)).
			Where("id = ?", masterEventModel.ID).
			Exists(context.Background()); err != nil {
			t.Error(err)
		} else if exists {
			t.Error("master event should not exist")
		}

		// the child event model should not exists
		if exists, err := bundb.NewSelect().
			Model((*model.ChildEvent)(nil)).
			Where("id = ?", childEventModel.ID).
			Exists(context.Background()); err != nil {
			t.Error(err)
		} else if exists {
			t.Error("child event should not exist")
		}

		// the attendee model should not exists
		if exists, err := bundb.NewSelect().
			Model((*model.Attendee)(nil)).
			Where("event_id = ?", childEventModel.ID).
			Exists(context.Background()); err != nil {
			t.Error(err)
		} else if exists {
			t.Error("attendee should not exist")
		}
	}()
}
