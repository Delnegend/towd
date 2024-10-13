package model

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/uptrace/bun"
)

type EventIDCtxKeyType string

const EventIDCtxKey EventIDCtxKeyType = "event-id"

type Event struct {
	bun.BaseModel `bun:"table:events"`

	ID          string `bun:"id,pk"`           // required
	Summary     string `bun:"summary,notnull"` // required
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDateUnixUTC int64 `bun:"start_date,notnull"` // required
	EndDateUnixUTC   int64 `bun:"end_date,notnull"`   // required
	IsWholeDay       bool  `bun:"is_whole_day"`

	CreatedAt int64 `bun:"created_at,notnull"`
	UpdatedAt int64 `bun:"updated_at"`
	Sequence  int   `bun:"sequence"`

	CalendarID string `bun:"calendar_id,notnull"` // required
	ChannelID  string `bun:"channel_id,notnull"`  // required

	Attendees        []*Attendee       `bun:"rel:has-many,join:id=event_id"`
	Calendar         *Calendar         `bun:"rel:belongs-to,join:calendar_id=channel_id"`
	ExternalCalendar *ExternalCalendar `bun:"rel:belongs-to,join:calendar_id=id"`
	NotificationSent bool              `bun:"notification_sent"` // required
}

func (e *Event) Upsert(ctx context.Context, db bun.IDB) error {
	switch {
	case e.ID == "":
		return fmt.Errorf("(*Event).Upsert: event id is blank")
	case e.Summary == "":
		return fmt.Errorf("(*Event).Upsert: summary is blank")
	case e.StartDateUnixUTC == 0:
		return fmt.Errorf("(*Event).Upsert: start date is blank")
	case e.EndDateUnixUTC == 0:
		return fmt.Errorf("(*Event).Upsert: end date is blank")
	case e.StartDateUnixUTC > e.EndDateUnixUTC:
		return fmt.Errorf("(*Event).Upsert: start date must be before end date")
	case e.URL != "":
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("(*Event).Upsert: url is invalid: %w", err)
		}
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().UTC().Unix()
	}
	startDateDate := time.Unix(e.StartDateUnixUTC, 0)
	if startDateDate.Hour() == 0 &&
		startDateDate.Minute() == 0 &&
		startDateDate.Second() == 0 {
		e.IsWholeDay = true
	}

	if _, err := db.NewInsert().
		Model(e).
		On("CONFLICT (id) DO UPDATE").
		Set("summary = EXCLUDED.summary").
		Set("description = EXCLUDED.description").
		Set("location = EXCLUDED.location").
		Set("url = EXCLUDED.url").
		Set("organizer = EXCLUDED.organizer").
		Set("start_date = EXCLUDED.start_date").
		Set("end_date = EXCLUDED.end_date").
		Set("is_whole_day = EXCLUDED.is_whole_day").
		Set("created_at = EXCLUDED.created_at").
		Set("sequence = EXCLUDED.sequence").
		Set("calendar_id = EXCLUDED.calendar_id").
		Set("channel_id = EXCLUDED.channel_id").
		Set("notification_sent = EXCLUDED.notification_sent").
		Exec(ctx); err != nil {
		return fmt.Errorf("(*Event).Upsert: %w", err)
	}

	return nil
}

func (e *Event) ToDiscordEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       e.Summary,
		Description: e.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Start Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.StartDateUnixUTC),
				Inline: true,
			},
			{
				Name:   "End Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.EndDateUnixUTC),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: e.ID,
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

	if len(e.Attendees) > 0 {
		attendeeStr := make([]string, len(e.Attendees))
		for i, attendee := range e.Attendees {
			attendeeStr[i] = attendee.Data
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Attendees",
			Value: strings.Join(attendeeStr, ", "),
		})
	}

	return embed
}

type DiffEvent struct {
	Title       string
	Description string
	StartDate   string
	EndDate     string
	Location    string
	URL         string
	Attendees   string
}

func (e *Event) Diff(otherEvent *Event) DiffEvent {
	diff := DiffEvent{}

	switch newExist, oldExist, theSame := otherEvent.Summary != "", e.Summary != "", otherEvent.Summary == e.Summary; {
	case newExist && oldExist && !theSame:
		diff.Title = fmt.Sprintf("%s `[old value: %s]`", otherEvent.Summary, e.Summary)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.Title = fmt.Sprintf("%s `[unchanged]`", e.Summary)
	case newExist && !oldExist:
		diff.Title = fmt.Sprintf("%s `[old value: None]`", otherEvent.Summary)
	default:
		diff.Title = "None `[unchanged]`"
	}

	switch newExist, oldExist, theSame := otherEvent.Description != "", e.Description != "", otherEvent.Description == e.Description; {
	case newExist && oldExist && !theSame:
		diff.Description = fmt.Sprintf("%s `[old value: %s]`", otherEvent.Description, e.Description)
	case newExist && !oldExist:
		diff.Description = fmt.Sprintf("%s `[old value: None]`", otherEvent.Description)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.Description = fmt.Sprintf("%s `[unchanged]`", e.Description)
	default:
		diff.Description = "None `[unchanged]`"
	}

	switch newExist, oldExist, theSame := otherEvent.StartDateUnixUTC != 0, e.StartDateUnixUTC != 0, otherEvent.StartDateUnixUTC == e.StartDateUnixUTC; {
	case newExist && oldExist && !theSame:
		diff.StartDate = fmt.Sprintf("<t:%d:f> `[old value]` <t:%d:f>", otherEvent.StartDateUnixUTC, e.StartDateUnixUTC)
	case newExist && !oldExist:
		diff.StartDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", otherEvent.StartDateUnixUTC)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.StartDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", e.StartDateUnixUTC)
	default:
		diff.StartDate = "None `[unchanged]`"
	}

	switch newExist, oldExist, theSame := otherEvent.EndDateUnixUTC != 0, e.EndDateUnixUTC != 0, otherEvent.EndDateUnixUTC == e.EndDateUnixUTC; {
	case newExist && oldExist && !theSame:
		diff.EndDate = fmt.Sprintf("<t:%d:f> `%d [old value]` <t:%d:f>", otherEvent.EndDateUnixUTC, otherEvent.EndDateUnixUTC, e.EndDateUnixUTC)
	case newExist && !oldExist:
		diff.EndDate = fmt.Sprintf("<t:%d:f> `[old value: None]`", otherEvent.EndDateUnixUTC)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.EndDate = fmt.Sprintf("<t:%d:f> `[unchanged]`", e.EndDateUnixUTC)
	default:
		diff.EndDate = "None `[unchanged]`"
	}

	switch newExist, oldExist, theSame := otherEvent.URL != "", e.URL != "", otherEvent.URL == e.URL; {
	case newExist && oldExist && !theSame:
		diff.URL = fmt.Sprintf("%s `[old value: %s]`", otherEvent.URL, e.URL)
	case newExist && !oldExist:
		diff.URL = fmt.Sprintf("%s `[old value: None]`", otherEvent.URL)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.URL = fmt.Sprintf("%s `[unchanged]`", e.URL)
	default:
		diff.URL = "None `[unchanged]`"
	}

	switch newExist, oldExist, theSame := otherEvent.Location != "", e.Location != "", otherEvent.Location == e.Location; {
	case newExist && oldExist && !theSame:
		diff.Location = fmt.Sprintf("%s `[old value: %s]`", otherEvent.Location, e.Location)
	case newExist && !oldExist:
		diff.Location = fmt.Sprintf("%s `[old value: None]`", otherEvent.Location)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.Location = fmt.Sprintf("%s `[unchanged]`", e.Location)
	default:
		diff.Location = "None `[unchanged]`"
	}

	oldAttendees := func() string {
		attendees := make([]string, len(e.Attendees))
		for i, attendee := range e.Attendees {
			attendees[i] = attendee.Data
		}
		return strings.Join(attendees, ", ")
	}()
	newAttendees := func() string {
		var attendees []string
		for _, attendeeModel := range otherEvent.Attendees {
			attendees = append(attendees, attendeeModel.Data)
		}
		return strings.Join(attendees, ", ")
	}()
	switch newExist, oldExist, theSame := newAttendees != "", oldAttendees != "", newAttendees == oldAttendees; {
	case newExist && oldExist && !theSame:
		diff.Attendees = fmt.Sprintf("%s `[old value: %s]`", newAttendees, oldAttendees)
	case newExist && !oldExist:
		diff.Attendees = fmt.Sprintf("%s `[old value: None]`", newAttendees)
	case (!newExist && oldExist) || (newExist && theSame):
		diff.Attendees = fmt.Sprintf("%s `[unchanged]`", oldAttendees)
	default:
		diff.Attendees = "None `[unchanged]`"
	}

	return diff
}
