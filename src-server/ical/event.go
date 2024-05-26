package ical

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xyedo/rrule"
)

type (
	EventStatus       string
	EventTransparency string
)

const (
	EventStatusConfirmed EventStatus = "CONFIRMED"
	EventStatusTentative EventStatus = "TENTATIVE"
	EventStatusCancelled EventStatus = "CANCELLED"

	EventTransparencyOpaque      EventTransparency = "OPAQUE"
	EventTransparencyTransparent EventTransparency = "TRANSPARENT"
)

type Event struct {
	id          string // required
	summary     string // required
	description string
	location    string
	url         string

	status       EventStatus
	transparency EventTransparency

	startDate time.Time // required
	endDate   time.Time // required
	wholeDay  bool

	attendee  []Attendee
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

		attendee: make([]Attendee, 0),
		exdate:   make([]time.Time, 0),
		rdate:    make([]time.Time, 0),
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
func (e *Event) GetStatus() EventStatus {
	return e.status
}
func (e *Event) GetTransparency() EventTransparency {
	return e.transparency
}
func (e *Event) GetStartDate() time.Time {
	return e.startDate
}
func (e *Event) GetEndDate() time.Time {
	return e.endDate
}
func (e *Event) GetAttendee() []Attendee {
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
		return fmt.Errorf("invalid URL")
	}
	e.hasModified()
	e.url = url_
	return nil
}
func (e *Event) SetStatus(status EventStatus) {
	e.hasModified()
	e.status = status
}
func (e *Event) SetTransparency(transparency EventTransparency) {
	e.hasModified()
	e.transparency = transparency
}
func (e *Event) SetStartDate(startDate time.Time) error {
	if !e.endDate.IsZero() && startDate.After(e.endDate) {
		return fmt.Errorf("start date is after end date")
	}
	e.hasModified()
	e.startDate = startDate
	return nil
}
func (e *Event) SetEndDate(endDate time.Time) error {
	if !e.startDate.IsZero() && endDate.Before(e.startDate) {
		return fmt.Errorf("end date is before start date")
	}
	e.hasModified()
	e.endDate = endDate
	return nil
}

func (e *Event) AddAttendee(attendee Attendee) {
	e.hasModified()
	if e.attendee == nil {
		e.attendee = make([]Attendee, 0)
	}
	e.attendee = append(e.attendee, attendee)
}
func (e *Event) RemoveAttendee(attendeeCn AttendeeCalAdrr) error {
	if e.attendee == nil {
		return fmt.Errorf("attendee is empty")
	}
	e.hasModified()
	for i, a := range e.attendee {
		if a.GetCN() == attendeeCn {
			e.attendee = append(e.attendee[:i], e.attendee[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("attendee not found")
}
func (e *Event) SetOrganizer(organizer string) {
	e.hasModified()
	e.organizer = organizer
}
func (e *Event) SetRRule(rrule_ *rrule.RRule) error {
	if rrule_ == nil {
		return fmt.Errorf("rrule is nil")
	}
	e.hasModified()
	e.rrule = rrule_
	return nil
}
func (e *Event) AddExDate(exdate time.Time) error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	e.hasModified()
	e.exdate = append(e.exdate, exdate)
	return nil
}
func (e *Event) RemoveExDate(exdate time.Time) error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	for i, d := range e.exdate {
		if d == exdate {
			e.hasModified()
			e.exdate = append(e.exdate[:i], e.exdate[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("exdate not found")
}
func (e *Event) ClearExDate() error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	e.hasModified()
	e.exdate = make([]time.Time, 0)
	return nil
}
func (e *Event) AddRDate(rdate time.Time) error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	e.hasModified()
	e.rdate = append(e.rdate, rdate)
	return nil
}
func (e *Event) RemoveRDate(rdate time.Time) error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	for i, d := range e.rdate {
		if d == rdate {
			e.hasModified()
			e.rdate = append(e.rdate[:i], e.rdate[i+1:]...)
			return nil
		}
	}
	return nil
}
func (e *Event) ClearRDate() error {
	if e.rrule == nil {
		return fmt.Errorf(errEventNotRecursive)
	}
	e.hasModified()
	e.rdate = make([]time.Time, 0)
	return nil
}
func (e *Event) SetRecurrenceID(recurrenceID time.Time) error {
	// TODO: check if match any recurrence rule
	e.hasModified()
	e.recurrenceID = recurrenceID
	return nil
}

// #endregion

func (e *Event) Validate() error {
	if e.id == "" {
		return fmt.Errorf("id not initialized")
	}
	if e.summary == "" {
		return fmt.Errorf("summary is missing")
	}
	if e.startDate.IsZero() {
		return fmt.Errorf("start date is missing")
	}
	if e.endDate.IsZero() {
		return fmt.Errorf("end date is missing")
	}

	recurrenceIDExist := !e.recurrenceID.IsZero()
	rruleExist := e.rrule != nil
	if recurrenceIDExist && rruleExist {
		return fmt.Errorf("recurrence-id and rrule cannot be used together")
	}

	if recurrenceIDExist && (len(e.rdate)+len(e.exdate)) > 0 {
		return fmt.Errorf("recurrence-id and rdate/exdate cannot be used together")
	}

	return nil
}

func (e *Event) Marshal() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}

	if e.isModified {
		e.isModified = false
		e.sequence++
	}

	result := make([]string, 0)

	result = append(result, "BEGIN:VEVENT")
	result = append(result, "UID:"+e.id)
	result = append(result, "SUMMARY:"+e.summary)
	if e.description != "" {
		result = append(result, "DESCRIPTION:"+e.description)
	}
	if e.location != "" {
		result = append(result, "LOCATION:"+e.location)
	}
	if e.url != "" {
		result = append(result, "URL:"+e.url)
	}

	var timeFormat string
	if e.wholeDay {
		timeFormat = "20060102"
	} else {
		timeFormat = "20060102T150405Z"
	}

	result = append(result, "DTSTART:"+e.startDate.Format(timeFormat))
	result = append(result, "DTEND:"+e.endDate.Format(timeFormat))

	result = append(result, "CREATED:"+e.createdAt.Format("20060102T150405Z"))
	if !e.updatedAt.IsZero() {
		result = append(result, "LAST-MODIFIED:"+e.updatedAt.Format("20060102T150405Z"))
	}
	if e.sequence >= 0 {
		result = append(result, fmt.Sprintf("SEQUENCE:%d", e.sequence))
	}
	if e.rrule != nil {
		result = append(result, "RRULE:"+e.rrule.String())
	}
	for _, exdate := range e.exdate {
		result = append(result, "EXDATE:"+exdate.Format(timeFormat))
	}
	for _, rdate := range e.rdate {
		result = append(result, "RDATE:"+rdate.Format(timeFormat))
	}
	if !e.recurrenceID.IsZero() {
		result = append(result, "RECURRENCE-ID:"+e.recurrenceID.Format(timeFormat))
	}
	result = append(result, fmt.Sprintf("DTSTAMP:%s", time.Now().Format("20060102T150405Z")))
	result = append(result, "END:VEVENT")

	return strings.Join(result, "\n"), nil
}
