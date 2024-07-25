package model

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"towd/src-server/ical/event"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

type DeletedChildEventIDsCtxKeyType string

const DeletedChildEventIDsCtxKey DeletedChildEventIDsCtxKeyType = "child-event-ids"

type ChildEvent struct {
	bun.BaseModel `bun:"table:child_events"`

	// id
	// - must be the same as the master event id
	// recurrence id
	// - act as exdates (it match any of the dates in the rrule set)
	// - fill in the excluded date with another event
	ID           string `bun:"id,notnull"`
	RecurrenceID int64  `bun:"recurrence_id,notnull"`

	Summary     string `bun:"summary,notnull"`
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDate int64 `bun:"start_date,notnull"`
	EndDate   int64 `bun:"end_date,notnull"`

	CreatedAt int64 `bun:"created_at,notnull"`
	UpdatedAt int64 `bun:"updated_at,notnull"`
	Sequence  int   `bun:"sequence"`

	Event *MasterEvent `bun:"rel:belongs-to,join:id=id"`
}

var _ bun.AfterDeleteHook = (*ChildEvent)(nil)

func (c *ChildEvent) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("ChildEvent.AfterDelete: db is nil")
	}

	switch deletedChildEventID := ctx.Value(DeletedChildEventIDsCtxKey).(type) {
	case string:
		if deletedChildEventID == "" {
			return fmt.Errorf("ChildEvent.AfterDelete: deletedChildEventID is blank")
		}

		// get the going-to-be-deleted attendee ids before deleting them
		deletedAttendeeIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*Attendee)(nil)).
			Column("event_id").
			Where("event_id = ?", deletedChildEventID).
			Scan(ctx, &deletedAttendeeIDs); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't get attendee ids: %w", err)
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id = ?", deletedChildEventID).
			Exec(ctx); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't delete attendees: %w", err)
		}
	case []string:
		if len(deletedChildEventID) == 0 {
			return fmt.Errorf("ChildEvent.AfterDelete: deletedChildEventID is empty")
		}

		// get the going-to-be-deleted attendee ids before deleting them
		deletedAttendeeIDs := []string{}
		if err := query.DB().NewSelect().
			Model((*Attendee)(nil)).
			Column("event_id").
			Where("event_id IN (?)", bun.In(deletedChildEventID)).
			Scan(ctx, &deletedAttendeeIDs); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't get attendee ids: %w", err)
		}

		// rm related attendees
		if _, err := query.DB().NewDelete().
			Model((*Attendee)(nil)).
			Where("event_id IN (?)", bun.In(deletedChildEventID)).
			Exec(context.WithValue(ctx, AttendeeIDCtxKey, deletedAttendeeIDs)); err != nil {
			return fmt.Errorf("ChildEvent.AfterDelete: can't delete attendees: %w", err)
		}
	case nil:
		return fmt.Errorf("ChildEvent.AfterDelete: child event ids is nil")
	default:
		return fmt.Errorf("ChildEvent.AfterDelete: wrong deletedChildEventID type | type=%T", deletedChildEventID)
	}

	return nil
}

func (c *ChildEvent) FromIcal(
	ctx context.Context,
	db bun.IDB,
	childEvent *event.ChildEvent,
) error {
	if db == nil {
		return fmt.Errorf("FromIcalChildEventToDB: db is nil")
	}

	c.ID = childEvent.GetID()
	c.RecurrenceID = childEvent.GetRecurrenceID().Unix()

	c.Summary = childEvent.GetSummary()
	c.Description = childEvent.GetDescription()
	c.Location = childEvent.GetLocation()
	c.URL = childEvent.GetURL()
	c.Organizer = childEvent.GetOrganizer()

	c.StartDate = childEvent.GetStartDate().Unix()
	c.EndDate = childEvent.GetEndDate().Unix()

	c.CreatedAt = childEvent.GetCreatedAt().Unix()
	c.UpdatedAt = childEvent.GetUpdatedAt().Unix()
	c.Sequence = childEvent.GetSequence()

	return nil
}

func (e *ChildEvent) Upsert(ctx context.Context, db bun.IDB) error {
	// basic field validation
	switch {
	case e.Summary == "":
		return fmt.Errorf("ChildEvent.Upsert: summary is required")
	case e.RecurrenceID == 0:
		return fmt.Errorf("ChildEvent.Upsert: recurrence id is required")
	case e.CreatedAt == 0:
		return fmt.Errorf("ChildEvent.Upsert: created at is required")
	case e.UpdatedAt != 0 && e.UpdatedAt < e.CreatedAt:
		return fmt.Errorf("ChildEvent.Upsert: updated at must be after created at")
	case e.StartDate == 0:
		return fmt.Errorf("ChildEvent.Upsert: start date is required")
	case e.EndDate == 0:
		return fmt.Errorf("ChildEvent.Upsert: end date is required")
	case e.StartDate > e.EndDate:
		return fmt.Errorf("ChildEvent.Upsert: start date must be before end date")
	}
	if e.URL != "" {
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("ChildEvent.Upsert: %w", err)
		}
	}

	// check if master event exists
	exist, err := db.NewSelect().
		Model(&MasterEvent{}).
		Where("id = ?", e.ID).
		Exists(context.Background())
	if err != nil {
		return fmt.Errorf("ChildEvent.Upsert: %w", err)
	}
	if !exist {
		return fmt.Errorf("ChildEvent.Upsert: master event id not found")
	}

	// upsert to db
	if _, err := db.NewInsert().
		Model(e).
		Exec(ctx); err != nil {
		return fmt.Errorf("ChildEvent.Upsert: %w", err)
	}

	return nil
}

// Format the event to be sent to Discord
func (e *ChildEvent) ToDiscordEmbed(ctx context.Context, db bun.IDB) (*discordgo.MessageEmbed, error) {
	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Start Date",
			Value:  fmt.Sprintf("<t:%d:t>", e.StartDate),
			Inline: true,
		},
		{
			Name:   "End Date",
			Value:  fmt.Sprintf("<t:%d:t>", e.EndDate),
			Inline: true,
		},
	}

	if e.Location != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "Location",
			Value: e.Location,
		})
	}
	if e.URL != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "URL",
			Value: e.URL,
		})
	}
	attendees := make([]Attendee, 0)
	if err := db.NewSelect().
		Model(&attendees).
		Where("event_id = ?", e.ID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("MasterEvent.ToDiscordEmbed: %w", err)
	}
	if len(attendees) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: "Attendees",
			Value: strings.Join(func() []string {
				attendeeNames := make([]string, len(attendees))
				for i, attendee := range attendees {
					attendeeNames[i] = attendee.Data
				}
				return attendeeNames
			}(), ", "),
		})
	}

	return &discordgo.MessageEmbed{
		Title:       e.Summary,
		Description: e.Description,
		Author: &discordgo.MessageEmbedAuthor{
			Name: e.Organizer,
		},
		Fields: fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: e.ID,
		},
	}, nil
}
