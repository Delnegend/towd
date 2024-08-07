package event

import (
	"fmt"
	"log/slog"
	"strings"
	"towd/src-server/ical/utils"

	"github.com/xyedo/rrule"
)

// The normal event
type MasterEvent struct {
	EventInfo

	rrule       *rrule.Set
	exDates     []int64
	rDates      []int64
	childEvents []*ChildEvent
}

// Get the recurrence rule
func (e *MasterEvent) GetRRuleSet() *rrule.Set {
	return e.rrule
}

// Iterate over the exdates and apply a function to each
func (e *MasterEvent) IterateExDates(fn func(int64)) {
	for _, exDate := range e.exDates {
		fn(exDate)
	}
}

// Iterate over the rdates and apply a function to each
func (e *MasterEvent) IterateRDates(fn func(int64)) {
	for _, rDate := range e.rDates {
		fn(rDate)
	}
}

// Add a child event to the master event
func (e *MasterEvent) AddChildEvent(childEvent *ChildEvent) error {
	if e.rrule == nil {
		return fmt.Errorf("MasterEvent.AddChildEvent: master event does not have a rrule, child event cannot be added")
	}

	unixDatesFromRRule := func() map[int64]struct{} {
		unixDatesFromRRule := make(map[int64]struct{})
		for _, date := range e.rrule.All() {
			unixDatesFromRRule[date.UTC().Unix()] = struct{}{}
		}
		return unixDatesFromRRule
	}()

	if _, ok := unixDatesFromRRule[childEvent.GetRecurrenceID()]; !ok {
		return fmt.Errorf("MasterEvent.AddChildEvent: rec-id (%d) not in rrule (%s)", childEvent.GetRecurrenceID(), func() string {
			var parsedRRuleSet []string
			for date := range unixDatesFromRRule {
				parsedRRuleSet = append(parsedRRuleSet, fmt.Sprintf("%d", date))
			}
			return strings.Join(parsedRRuleSet, ",")
		}())
	}

	e.childEvents = append(e.childEvents, childEvent)
	return nil
}

// Iterate over the child events and apply a function to each
func (e *MasterEvent) IterateChildEvents(fn func(id string, event *ChildEvent) error) error {
	for _, childEvent := range e.childEvents {
		return fn(childEvent.GetID(), childEvent)
	}
	return nil
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
func (e *MasterEvent) ToIcal(writer func(string)) {
	// basic properties
	writer("BEGIN:VEVENT\n")
	if err := e.EventInfo.toIcal(writer); err != nil {
		slog.Warn("MasterEvent.ToIcal: can't write basic properties", "error", err)
		return
	}

	// recurrence
	if e.rrule != nil {
		writer("RRULE:" + e.rrule.GetRRule().String() + "\n")
	}
	for _, exdate := range e.exDates {
		writer("EXDATE:" + utils.Unix2Datetime(exdate) + "\n")
	}
	for _, rdate := range e.rDates {
		writer("RDATE:" + utils.Unix2Datetime(rdate) + "\n")
	}

	// child events
	for _, childEvent := range e.childEvents {
		writer("BEGIN:VEVENT\n")
		if err := childEvent.EventInfo.toIcal(writer); err != nil {
			slog.Warn("MasterEvent.ToIcal: can't write basic properties for child event", "error", err)
			return
		}
		writer("RECURRENCE-ID:" + utils.Unix2Datetime(childEvent.recurrenceID) + "\n")
		writer("END:VEVENT\n")
	}
}
