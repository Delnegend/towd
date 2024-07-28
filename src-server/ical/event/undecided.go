package event

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"towd/src-server/ical/structured"
	"towd/src-server/ical/utils"

	"github.com/google/uuid"
	"github.com/xyedo/rrule"
)

// Holds everything an event could possibly hold
type UndecidedEvent struct {
	EventInfo

	rruleSet     *rrule.Set
	exDate       []int64
	rDate        []int64
	recurrenceID int64
}

// Create a new undecided event with new UID
func NewUndecidedEvent() UndecidedEvent {
	return UndecidedEvent{
		EventInfo: EventInfo{
			id: uuid.NewString(),
		},
	}
}

// Set the event ID
func (e *UndecidedEvent) SetID(id string) *UndecidedEvent {
	e.id = id
	return e
}

// Set the event summary
func (e *UndecidedEvent) SetSummary(summary string) *UndecidedEvent {
	e.summary = summary
	return e
}

// Set the event description
func (e *UndecidedEvent) SetDescription(description string) *UndecidedEvent {
	e.description = description
	return e
}

// Set the event location.
// Returns itself for chaining.
func (e *UndecidedEvent) SetLocation(location string) *UndecidedEvent {
	e.location = location
	return e
}

// Set the event URL
func (e *UndecidedEvent) SetURL(url string) *UndecidedEvent {
	e.url = url
	return e
}

// Set the event start date
func (e *UndecidedEvent) SetStartDate(startDate int64) *UndecidedEvent {
	e.startDate = startDate
	return e
}

// Set the event end date
func (e *UndecidedEvent) SetEndDate(endDate int64) *UndecidedEvent {
	e.endDate = endDate
	return e
}

// Set the event created date
func (e *UndecidedEvent) SetCreatedAt(createdAt int64) *UndecidedEvent {
	e.createdAt = createdAt
	return e
}

// Set the event last modified date
func (e *UndecidedEvent) SetUpdatedAt(lastModified int64) *UndecidedEvent {
	e.updatedAt = lastModified
	return e
}

func (e *UndecidedEvent) AddAttendee(attendee structured.Attendee) *UndecidedEvent {
	e.attendee = append(e.attendee, attendee)
	return e
}

// Set the event organizer
func (e *UndecidedEvent) SetOrganizer(organizer string) *UndecidedEvent {
	e.organizer = organizer
	return e
}

// Add an alarm to the event
func (e *UndecidedEvent) AddAlarm(alarm structured.Alarm) *UndecidedEvent {
	e.alarm = append(e.alarm, alarm)
	return e
}

// Add a custom property to the event
func (e *UndecidedEvent) AddCustomProperty(property string) *UndecidedEvent {
	e.customProperties = append(e.customProperties, property)
	return e
}

// Set the event sequence
func (e *UndecidedEvent) SetSequence(sequence int) *UndecidedEvent {
	e.sequence = sequence
	return e
}

// Add an iCalendar property to the event.
// Unhandled properties will be stored in the customProperties array.
func (e *UndecidedEvent) AddIcalProperty(property string) error {
	// properties don't have regular key:value format
	switch {
	case strings.HasPrefix(property, "X-"):
		if e.customProperties == nil {
			e.customProperties = make([]string, 0)
		}
		e.customProperties = append(e.customProperties, property)
		return nil
	case strings.HasPrefix(property, "ATTENDEE"):
		attendee := structured.Attendee{}
		if err := attendee.FromIcal(property); err != nil {
			return err
		}
		e.attendee = append(e.attendee, attendee)
		return nil
	case strings.HasPrefix(property, "ORGANIZER"):
		e.organizer = strings.TrimPrefix(property, "ORGANIZER:")
		return nil
	case strings.HasPrefix(property, "ATTACH"):
		e.customProperties = append(e.customProperties, property)
		return nil
	case strings.HasPrefix(property, "DTSTART"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		if e.endDate != 0 && parsedDate > e.endDate {
			return fmt.Errorf("DTSTART must be before DTEND")
		}
		e.startDate = parsedDate
		return nil
	case strings.HasPrefix(property, "DTEND"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		if e.startDate != 0 && parsedDate < e.startDate {
			return fmt.Errorf("DTEND must be after DTSTART")
		}
		e.endDate = parsedDate
		return nil
	case strings.HasPrefix(property, "EXDATE"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		e.exDate = append(e.exDate, parsedDate)
		return nil
	case strings.HasPrefix(property, "DTSTAMP"):
		return nil
	case strings.HasPrefix(property, "CREATED"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		e.createdAt = parsedDate
		return nil
	case strings.HasPrefix(property, "LAST-MODIFIED"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		e.updatedAt = parsedDate
		return nil
	case strings.HasPrefix(property, "RDATE"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		e.rDate = append(e.rDate, parsedDate)
		return nil
	case strings.HasPrefix(property, "RECURRENCE-ID"):
		parsedDate, err := utils.IcalDatetimeToTime(property)
		if err != nil {
			return err
		}
		e.recurrenceID = parsedDate
		return nil
	}

	slice := strings.SplitN(property, ":", 2)
	if len(slice) != 2 {
		return nil
	}
	key := strings.ToUpper(strings.TrimSpace(slice[0]))
	val := strings.TrimSpace(slice[1])

	switch key {
	case "UID":
		e.id = val
	case "SUMMARY":
		e.summary = val
	case "DESCRIPTION":
		e.description = val
	case "LOCATION":
		e.location = val
	case "URL":
		if _, err := url.ParseRequestURI(val); err != nil {
			return fmt.Errorf("invalid URL")
		}
		e.url = val
	case "SEQUENCE":
		sequence, err := strconv.Atoi(val)
		if err != nil || sequence < 0 {
			return fmt.Errorf("invalid SEQUENCE")
		}
		e.sequence = sequence
	case "RRULE":
		switch {
		case e.startDate == 0:
			return fmt.Errorf("RRULE requires a start date")
		case e.endDate.IsZero():
			return fmt.Errorf("RRULE requires an end date")
		case e.recurrenceID != 0:
			return fmt.Errorf("RRULE and RECURRENCE-ID are mutually exclusive")
		}
		var sb strings.Builder
		sb.WriteString("DTSTART:" + e.startDate.Format("20060102T150405Z"))
		sb.WriteString("\nDTEND:" + e.endDate.Format("20060102T150405Z"))
		sb.WriteString("\nRRULE:" + val)

		rruleSet, err := rrule.StrToRRuleSet(sb.String())
		if err != nil {
			return err
		}
		e.rruleSet = rruleSet
	default:
		e.customProperties = append(e.customProperties, property)
	}
	return nil
}

// Convert the template event into a master or child event
func (e *UndecidedEvent) DecideEventType() (interface{}, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}

	switch {
	// expect to be a child event has a more strict condition
	case e.recurrenceID != 0 && e.rruleSet != nil:
		return nil, fmt.Errorf("seems like a child event, but rruleSet is set")
	case e.recurrenceID != 0 && len(e.exDate) > 0:
		return nil, fmt.Errorf("seems like a child event, but exdate is set")
	case e.recurrenceID != 0 && len(e.rDate) > 0:
		return nil, fmt.Errorf("seems like a child event, but rdate is set")

	case (e.rruleSet == nil && len(e.exDate) > 0):
		return nil, fmt.Errorf("exdate only works with recurring events")
	case (e.rruleSet == nil && len(e.rDate) > 0):
		return nil, fmt.Errorf("rdate only works with recurring events")

	case e.recurrenceID == 0:
		return MasterEvent{
			EventInfo: e.EventInfo,
			rrule:     e.rruleSet,
			exDates:   e.exDate,
			rDates:    e.rDate,
		}, nil

	// to be a child event has a more strict condition; the template event
	// needs to have a recurrence-id and must not have exdate, rdate or rruleSet
	// since all of them are master's properties, although they are optional
	case e.recurrenceID != 0 && (e.rruleSet == nil) &&
		(len(e.exDate) == 0) && (len(e.rDate) == 0):
		return ChildEvent{
			EventInfo:    e.EventInfo,
			recurrenceID: e.recurrenceID,
		}, nil
	default:
		return nil, fmt.Errorf("cannot decide event type")
	}
}
