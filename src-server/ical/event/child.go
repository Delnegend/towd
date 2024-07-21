package event

import "time"

// Event that overrides the recurrence rule of a normal event.
type ChildEvent struct {
	EventInfo

	recurrenceID time.Time
}

// Turn a ChildEvent into an UndecidedEvent for modification.
func (e *ChildEvent) ToUndecidedEvent() UndecidedEvent {
	return UndecidedEvent{
		EventInfo:    e.EventInfo,
		recurrenceID: e.recurrenceID,
	}
}

// Get the event recurrence ID.
func (e *ChildEvent) GetRecurrenceID() time.Time {
	return e.recurrenceID
}
