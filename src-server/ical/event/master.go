package event

import (
	"fmt"
	"strings"
	"time"
	"towd/src-server/ical/utils"

	"github.com/xyedo/rrule"
)

// The normal event
type MasterEvent struct {
	EventInfo

	rrule       *rrule.Set
	exDates     []time.Time
	rDates      []time.Time
	childEvents []*ChildEvent
}

// Get the recurrence rule
func (e *MasterEvent) GetRRuleSet() *rrule.Set {
	return e.rrule
}

// Iterate over the exdates and apply a function to each
func (e *MasterEvent) IterateExDates(fn func(time.Time)) {
	for _, exDate := range e.exDates {
		fn(exDate)
	}
}

// Iterate over the rdates and apply a function to each
func (e *MasterEvent) IterateRDates(fn func(time.Time)) {
	for _, rDate := range e.rDates {
		fn(rDate)
	}
}

// Add a child event to the master event
func (e *MasterEvent) AddChildEvent(childEvent *ChildEvent) error {
	if e.rrule == nil {
		return fmt.Errorf("MasterEvent.AddChildEvent: master event does not have a rrule, child event cannot be added")
	}

	rruleTimeSlice := e.rrule.All()
	for _, rruleTime := range rruleTimeSlice {
		if rruleTime.Equal(childEvent.GetRecurrenceID()) {
			e.childEvents = append(e.childEvents, childEvent)
			return nil
		}
	}

	return fmt.Errorf("MasterEvent.AddChildEvent: child event recurrence id not found in master event rrule")
}

// Iterate over the child events and apply a function to each
func (e *MasterEvent) IterateChildEvents(fn func(id string, event *ChildEvent)) {
	for _, childEvent := range e.childEvents {
		fn(childEvent.GetID(), childEvent)
	}
}

// Turn a MasterEvent into an UndecidedEvent for modification
func (e *MasterEvent) ToUndecidedEvent() UndecidedEvent {
	return UndecidedEvent{
		EventInfo: e.EventInfo,
		rruleSet:  e.rrule,
		exDate:    e.exDates,
		rDate:     e.rDates,
	}
}

// Convert the MasterEvent into an iCalendar string. This method is intended to
// be used internally only. Check the usage in the master.go file.
func (e *MasterEvent) ToIcal() (string, error) {
	var sb strings.Builder
	writer := utils.Split75wrapper(sb.WriteString)

	// basic properties
	writer("BEGIN:VEVENT\n")
	if err := e.EventInfo.toIcal(writer); err != nil {
		return "", err
	}

	// recurrence
	if e.rrule != nil {
		writer(fmt.Sprintf("RRULE:%s\n", e.rrule.String()))
	}
	for _, exdate := range e.exDates {
		exDateStr, err := utils.TimeToIcalDatetime(exdate)
		if err != nil {
			return "", err
		}
		writer(fmt.Sprintf("EXDATE:%s\n", exDateStr))
	}
	for _, rdate := range e.rDates {
		rdateStr, err := utils.TimeToIcalDatetime(rdate)
		if err != nil {
			return "", nil
		}
		writer(fmt.Sprintf("RDATE:%s\n", rdateStr))
	}

	// child events
	for _, childEvent := range e.childEvents {
		writer("BEGIN:VEVENT\n")
		if err := childEvent.EventInfo.toIcal(writer); err != nil {
			return "", err
		}
		recurrenceIDStr, err := utils.TimeToIcalDatetime(childEvent.recurrenceID)
		if err != nil {
			return "", err
		}
		writer("RECURRENCE-ID:" + recurrenceIDStr + "\n")
		writer("END:VEVENT\n")
	}

	return sb.String(), nil
}
