// Package `event` contains the `MasterEvent`, `ChildEvent` and `UndecidedEvent`
// structs, which are used to represent events in the calendar.
//
// The `MasterEvent` struct represents a single event in the calendar, while the
// `ChildEvent` struct represents a single instance of a recurring event. Both
// structs are immutable.
//
// To create a new event, use the `NewUndecidedEvent` function, then call the
// `DecideEventType` method to validate missing or invalid data and return a
// `MasterEvent` or `ChildEvent` struct. Example usage:
//
//	blankEvent := ical.NewUndecidedEvent()
//	blankEvent.summary = "My Event"
//	blankEvent.startDate = time.Now()
//	blankEvent.endDate = time.Now().Add(time.Hour * 24)
//	resultEvent, err := blankEvent.DecideEventType()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	switch resultEvent := resultEvent.(type) {
//	case ical.MasterEvent:
//	    // do something with the MasterEvent
//	case ical.ChildEvent:
//	    // do something with the ChildEvent
//	default:
//	    log.Fatal("can't decide event type")
//	}
//
// To modify an existing `MasterEvent` or `ChildEvent`, use the `UpdateEvent`

package event
