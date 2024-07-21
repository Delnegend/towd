package event

import (
	"fmt"
	"time"
	"towd/src-server/ical/structured"
	"towd/src-server/ical/utils"
)

// Purely for reusing the same property in all types of events.
// - Only getters are available as
//   - this struct is being used in UndecidedEvent, MasterEvent, and ChildEvent.
//   - MasterEvent and ChildEvent are immutable.
//   - UndecidedEvent is mutable.
//
// - DO NOT USE THIS STRUCT IN YOUR CODE. Instead use the
// `Calendar{}.NewUndecidedEvent()`
type EventInfo struct {
	id string

	summary     string
	description string
	location    string
	url         string
	startDate   time.Time
	endDate     time.Time
	createdAt   time.Time
	updatedAt   time.Time

	attendee         []structured.Attendee
	organizer        string
	alarm            []structured.Alarm
	sequence         int
	customProperties []string
}

// Get the event ID
func (e *EventInfo) GetID() string {
	return e.id
}

// Get the event summary
func (e *EventInfo) GetSummary() string {
	return e.summary
}

// Get the event description
func (e *EventInfo) GetDescription() string {
	return e.description
}

// Get the event location
func (e *EventInfo) GetLocation() string {
	return e.location
}

// Get the event URL
func (e *EventInfo) GetURL() string {
	return e.url
}

// Get the event start date
func (e *EventInfo) GetStartDate() time.Time {
	return e.startDate
}

// Get the event end date
func (e *EventInfo) GetEndDate() time.Time {
	return e.endDate
}

// Get the event updated date
func (e *EventInfo) GetCreatedAt() time.Time {
	return e.createdAt
}

// Get the event updated date
func (e *EventInfo) GetUpdatedAt() time.Time {
	return e.updatedAt
}

// Get the event attendees
func (e *EventInfo) GetAttendee() []structured.Attendee {
	return e.attendee
}

// Get the event organizer
func (e *EventInfo) GetOrganizer() string {
	return e.organizer
}

// Get the event alarms
func (e *EventInfo) GetAlarm() []structured.Alarm {
	return e.alarm
}

// Get the event custom properties
func (e *EventInfo) GetCustomProperties() []string {
	return e.customProperties
}

// Get the event sequence
func (e *EventInfo) GetSequence() int {
	return e.sequence
}

func (e *EventInfo) validate() error {
	switch {
	case e.summary == "":
		return fmt.Errorf("summary is missing")
	case e.startDate.IsZero():
		return fmt.Errorf("start date is missing")
	case e.endDate.IsZero():
		return fmt.Errorf("end date is missing")
	case !e.startDate.Equal(e.endDate) && e.startDate.After(e.endDate):
		return fmt.Errorf("start date must be before end date")
	case e.sequence < 0:
		return fmt.Errorf("sequence must be non-negative")
	default:
		return nil
	}
}

// Convert the EventInfo into an iCalendar string. This method is intended to be
// used internally only. Example usage:
//
//	var sb strings.Builder
//	writer := split75wrapper(sb.WriteString)
//	// ...
//	if err := event.toIcal(writer); err != nil {
//	    log.Fatal(err)
//	}
func (e *EventInfo) toIcal(writer func(string) (int, error)) error {
	if err := e.validate(); err != nil {
		return err
	}

	// basic properties
	writer(fmt.Sprintf("UID:%s\nSUMMARY:%s\n", e.id, e.summary))
	if e.description != "" {
		writer(fmt.Sprintf("DESCRIPTION:%s\n", e.description))
	}
	if e.location != "" {
		writer(fmt.Sprintf("LOCATION:%s\n", e.location))
	}
	if e.url != "" {
		writer(fmt.Sprintf("URL:%s\n", e.url))
	}

	// dates
	startDateStr, err := utils.TimeToIcalDatetime(e.startDate)
	if err != nil {
		return err
	}
	writer(fmt.Sprintf("DTSTART:%s\n", startDateStr))
	endDateStr, err := utils.TimeToIcalDatetime(e.endDate)
	if err != nil {
		return err
	}
	writer(fmt.Sprintf("DTEND:%s\n", endDateStr))
	writer(fmt.Sprintf("DTSTAMP:%s\n", time.Now().Format("20060102T150405Z")))
	writer(fmt.Sprintf("CREATED:%s\n", time.Now().Format("20060102T150405Z")))
	if !e.updatedAt.IsZero() {
		writer(fmt.Sprintf("LAST-MODIFIED:%s\n", e.updatedAt.Format("20060102T150405Z")))
	}

	// involved people
	if len(e.attendee) > 0 {
		for _, attendee := range e.attendee {
			if err := attendee.ToIcal(writer); err != nil {
				return err
			}
		}
	}
	if e.organizer != "" {
		writer(fmt.Sprintf("ORGANIZER:%s\n", e.organizer))
	}

	// miscellaneous
	for _, alarm := range e.alarm {
		if err := alarm.ToIcal(writer); err != nil {
			return err
		}
	}
	if e.sequence > 0 {
		writer(fmt.Sprintf("SEQUENCE:%d\n", e.sequence))
	}

	// custom properties
	for _, customProperty := range e.customProperties {
		writer(customProperty)
	}

	return nil
}
