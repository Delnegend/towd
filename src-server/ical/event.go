package ical

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xyedo/rrule"
)

type Event struct {
	id          string // required
	summary     string // required
	description string
	location    string
	url         string

	startDate time.Time // required
	endDate   time.Time // required
	wholeDay  bool

	attendee  []string
	organizer string // required

	createdAt time.Time // required
	updatedAt time.Time
	sequence  int

	rrule        *rrule.RRule
	exdate       []time.Time
	rdate        []time.Time
	recurrenceID time.Time

	// sequence only update once the event is serialized, not after each modification
	isModified bool
}

// initialize a new event struct
func NewEvent() Event {
	return Event{
		id:         uuid.NewString(),
		createdAt:  time.Now(),
		sequence:   -1,
		wholeDay:   false,
		isModified: false,
	}
}

// set the isModified flag to true and update the updatedAt field
func (e *Event) hasModified() {
	e.isModified = true
	e.updatedAt = time.Now()
}

// #region Getters

func (e *Event) GetId() string {
	return e.id
}

func (e *Event) GetSummary() string {
	return e.summary
}

func (e *Event) GetDescription() string {
	return e.description
}

func (e *Event) GetLocation() string {
	return e.location
}

func (e *Event) GetUrl() string {
	return e.url
}

func (e *Event) GetStartDate() time.Time {
	return e.startDate
}

func (e *Event) GetEndDate() time.Time {
	return e.endDate
}

func (e *Event) GetAttendee() []string {
	return e.attendee
}

func (e *Event) GetOrganizer() string {
	return e.organizer
}

func (e *Event) GetCreatedAt() time.Time {
	return e.createdAt
}

func (e *Event) GetUpdatedAt() time.Time {
	return e.updatedAt
}

func (e *Event) GetSequence() int {
	return e.sequence
}

func (e *Event) GetRRule() *rrule.RRule {
	return e.rrule
}

func (e *Event) GetExDate() []time.Time {
	return e.exdate
}

func (e *Event) GetRDate() []time.Time {
	return e.rdate
}

func (e *Event) GetRecurrenceID() time.Time {
	return e.recurrenceID
}

// #endregion

// #region Setters

func (e *Event) SetID(id string) {
	e.hasModified()
	e.id = id
}

func (e *Event) SetSummary(summary string) {
	e.hasModified()
	e.summary = summary
}

func (e *Event) SetDescription(description string) {
	e.description = description
}

func (e *Event) SetLocation(location string) {
	e.hasModified()
	e.location = location
}

func (e *Event) SetUrl(url_ string) error {
	if _, err := url.ParseRequestURI(url_); err != nil {
		return errors.New("invalid URL")
	}
	e.hasModified()
	e.url = url_
	return nil
}

func (e *Event) SetStartDate(startDate time.Time) error {
	if !e.endDate.IsZero() && startDate.After(e.endDate) {
		return errors.New("start date is after end date")
	}
	e.hasModified()
	e.startDate = startDate
	return nil
}

func (e *Event) SetEndDate(endDate time.Time) error {
	if !e.startDate.IsZero() && endDate.Before(e.startDate) {
		return errors.New("end date is before start date")
	}
	e.hasModified()
	e.endDate = endDate
	return nil
}

func (e *Event) SetWholeDay(wholeDay bool) {
	e.hasModified()
	e.wholeDay = wholeDay
}

func (e *Event) ClearStartEndDate() {
	e.hasModified()
	e.startDate = time.Time{}
	e.endDate = time.Time{}
}

func (e *Event) SetAttendee(attendee []string) {
	e.hasModified()
	e.attendee = attendee
}

func (e *Event) SetOrganizer(organizer string) {
	e.hasModified()
	e.organizer = organizer
}

func (e *Event) SetRRule(rrule_ string) error {
	e.hasModified()
	result, err := rrule.StrToRRule(rrule_)
	if err != nil {
		return err
	}
	e.rrule = result
	return nil
}

func (e *Event) SetExDate(exdate []time.Time) {
	e.hasModified()
	e.exdate = exdate
}

func (e *Event) SetRDate(rdate []time.Time) {
	e.rdate = rdate
}

func (e *Event) SetRecurrenceID(recurrenceID time.Time) {
	e.hasModified()
	e.recurrenceID = recurrenceID
}

// #endregion

