package event

import (
	"fmt"
	"strconv"
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
	startDate   int64
	endDate     int64
	createdAt   int64
	updatedAt   int64

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
func (e *EventInfo) GetStartDate() int64 {
	return e.startDate
}

// Get the event end date
func (e *EventInfo) GetEndDate() int64 {
	return e.endDate
}

// Get the event updated date
func (e *EventInfo) GetCreatedAt() int64 {
	return e.createdAt
}

// Get the event updated date
func (e *EventInfo) GetUpdatedAt() int64 {
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
	case e.startDate == 0:
		return fmt.Errorf("start date is missing")
	case e.endDate != 0 && e.startDate > e.endDate:
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
func (e *EventInfo) toIcal(writer func(string)) error {
	if err := e.validate(); err != nil {
		return err
	}

	// basic properties
	writer("UID:" + e.id + "\n")
	writer("SUMMARY:" + e.summary + "\n")
	if e.description != "" {
		writer("DESCRIPTION:" + e.description + "\n")
	}
	if e.location != "" {
		writer("LOCATION:" + e.location + "\n")
	}
	if e.url != "" {
		writer("URL:" + e.url + "\n")
	}

	// dates
	startDateStr, err := utils.TimeToIcalDatetime(e.startDate)
	if err != nil {
		return err
	}
	writer("DTSTART:" + startDateStr + "\n")
	endDateStr, err := utils.TimeToIcalDatetime(e.endDate)
	if err != nil {
		return err
	}
	writer("DTEND:" + endDateStr + "\n")
	writer("DTSTAMP:" + time.Now().Format("20060102T150405Z") + "\n")
	writer("CREATED:" + time.Now().Format("20060102T150405Z") + "\n")
	if e.updatedAt != 0 {
		updatedAt := time.Unix(e.updatedAt, 0).Format("20060102T150405Z")
		writer("LAST-MODIFIED:" + updatedAt + "\n")
	}

	// involved people
	if len(e.attendee) > 0 {
		for _, attendee := range e.attendee {
			attendee.ToIcal(writer)
		}
	}
	if e.organizer != "" {
		writer("ORGANIZER:" + e.organizer + "\n")
	}

	// miscellaneous
	for _, alarm := range e.alarm {
		alarm.ToIcal(writer)
	}
	if e.sequence > 0 {
		writer("SEQUENCE:" + strconv.Itoa(e.sequence) + "\n")
	}

	// custom properties
	for _, customProperty := range e.customProperties {
		writer(customProperty + "\n")
	}

	return nil
}
