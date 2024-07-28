package event

// Event that overrides the recurrence rule of a normal event.
type ChildEvent struct {
	EventInfo

	recurrenceID int64
}

// Turn a ChildEvent into an UndecidedEvent for modification.
func (e *ChildEvent) ToUndecidedEvent() UndecidedEvent {
	return UndecidedEvent{
		EventInfo:    e.EventInfo,
		recurrenceID: e.recurrenceID,
	}
}

// Get the event recurrence ID.
func (e *ChildEvent) GetRecurrenceID() int64 {
	return e.recurrenceID
}
