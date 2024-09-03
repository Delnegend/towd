package model

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

type ChildEventIDsCtxKeyType string

const ChildEventIDCtxKey ChildEventIDsCtxKeyType = "child-event-ids"

type ChildEvent struct {
	bun.BaseModel `bun:"table:child_events"`

	ID            string `bun:"id,pk"`                   // required
	MasterEventID string `bun:"master_event_id,notnull"` // required
	// a.k.a one of the parsed dates from master event's rrule set
	RecurrenceID int64 `bun:"recurrence_id,notnull"` // required

	Summary     string `bun:"summary,notnull"` // 	required
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDate int64 `bun:"start_date,notnull"` // required
	EndDate   int64 `bun:"end_date,notnull"`   // required

	CreatedAt int64 `bun:"created_at,notnull"` // required
	UpdatedAt int64 `bun:"updated_at"`
	Sequence  int   `bun:"sequence"`

	CalendarID string `bun:"calendar_id,notnull"` // required
	ChannelID  string `bun:"channel_id,notnull"`  // required

	Event    *MasterEvent `bun:"rel:belongs-to,join:master_event_id=id"`
	Calendar *Calendar    `bun:"rel:belongs-to,join:calendar_id=id"`
}

var _ bun.AfterDeleteHook = (*ChildEvent)(nil)

// Cleanup attendees after child event is deleted
func (c *ChildEvent) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("(*ChildEvent).AfterDelete: db is nil")
	}

	// getting just-deleted-child-event-ids from context
	childEventIDs := make([]string, 0)
	switch childEventID := ctx.Value(ChildEventIDCtxKey).(type) {
	case string:
		if childEventID == "" {
			return fmt.Errorf("(*ChildEvent).AfterDelete: deletedChildEventID is blank")
		}
		childEventIDs = append(childEventIDs, childEventID)
	case []string:
		if len(childEventID) == 0 {
			return nil
		}
		childEventIDs = append(childEventIDs, childEventID...)
	case nil:
		return fmt.Errorf("(*ChildEvent).AfterDelete: child event ids is nil")
	default:
		return fmt.Errorf("(*ChildEvent).AfterDelete: wrong childEventID type | type=%T", childEventID)
	}

	// delete all related Attendee models
	if _, err := query.DB().NewDelete().
		Model((*Attendee)(nil)).
		Where("event_id IN (?)", bun.In(childEventIDs)).
		Exec(ctx); err != nil {
		return fmt.Errorf("(*ChildEvent).AfterDelete: can't delete attendees: %w", err)
	}

	return nil
}

// Upsert the child event to the database
func (e *ChildEvent) Upsert(ctx context.Context, db bun.IDB) error {
	// #region - basic field validation
	switch {
	case e.Summary == "":
		return fmt.Errorf("(*ChildEvent).Upsert: summary is required")
	case e.RecurrenceID == 0:
		return fmt.Errorf("(*ChildEvent).Upsert: recurrence id is required")
	case e.CreatedAt == 0:
		return fmt.Errorf("(*ChildEvent).Upsert: created at is required")
	case e.UpdatedAt != 0 && e.UpdatedAt < e.CreatedAt:
		return fmt.Errorf("(*ChildEvent).Upsert: updated at must be after created at")
	case e.StartDate == 0:
		return fmt.Errorf("(*ChildEvent).Upsert: start date is required")
	case e.EndDate == 0:
		return fmt.Errorf("(*ChildEvent).Upsert: end date is required")
	case e.StartDate > e.EndDate:
		return fmt.Errorf("(*ChildEvent).Upsert: start date must be before end date")
	}
	if e.URL != "" {
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("(*ChildEvent).Upsert: %w", err)
		}
	}
	// #endregion

	// #region - check if master event exists
	exist, err := db.NewSelect().
		Model((*MasterEvent)(nil)).
		Where("id = ?", e.MasterEventID).
		Exists(context.Background())
	if err != nil {
		return fmt.Errorf("(*ChildEvent).Upsert: %w", err)
	}
	if !exist {
		return fmt.Errorf("(*ChildEvent).Upsert: master event id not found")
	}
	// #endregion

	// #region - check if from a read-only calendar
	masterEventModal := new(MasterEvent)
	if err := db.NewSelect().
		Model(masterEventModal).
		Where("id = ?", e.MasterEventID).
		Scan(ctx, masterEventModal); err != nil {
		return fmt.Errorf("(*ChildEvent).Upsert: can't get master event: %w", err)
	}
	calendarModal := new(Calendar)
	if err := db.NewSelect().
		Model(calendarModal).
		Where("id = ?", masterEventModal.CalendarID).
		Scan(ctx, calendarModal); err != nil {
		return fmt.Errorf("(*ChildEvent).Upsert: can't get calendar: %w", err)
	}
	if calendarModal.Url != "" {
		return fmt.Errorf("(*ChildEvent).Upsert: this event is from a read-only calendar")
	}
	// #endregion

	// upsert to db
	if _, err := db.NewInsert().
		Model(e).
		Exec(ctx); err != nil {
		return fmt.Errorf("(*ChildEvent).Upsert: %w", err)
	}

	return nil
}

// Turn the child event into a discord embeded message.
// It's just a single embed though, but return as a slice for easier processing.
func (e *ChildEvent) ToDiscordEmbed(ctx context.Context, db bun.IDB) []*discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       e.Summary,
		Description: e.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Start Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.StartDate),
				Inline: true,
			},
			{
				Name:   "End Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.EndDate),
				Inline: true,
			},
			{
				Name:  "Master Event's ID",
				Value: fmt.Sprintf("<t:%s:f>", e.MasterEventID),
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d", e.RecurrenceID),
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name: e.Organizer,
		},
	}
	if e.Location != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Location",
			Value: e.Location,
		})
	}
	if e.URL != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "URL",
			Value: e.URL,
		})
	}
	attendees := func() []string {
		var attendeeModels []Attendee
		if err := db.NewSelect().
			Model(&attendeeModels).
			Where("event_id = ?", e.RecurrenceID).
			Scan(ctx, &attendeeModels); err != nil {
			slog.Warn("can't get attendees", "where", "(*ChildEvent).ToDiscordEmbed", "error", err)
			return []string{}
		}
		attendees := make([]string, len(attendeeModels))
		for i, attendee := range attendeeModels {
			attendees[i] = attendee.Data
		}
		return attendees
	}()
	if len(attendees) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Invitees",
			Value: strings.Join(attendees, ", "),
		})
	}

	return []*discordgo.MessageEmbed{embed}
}
