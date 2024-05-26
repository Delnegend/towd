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
	alarm     []Alarm
	attach    string

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
func (e *Event) GetAlarm() []Alarm {
	return e.alarm
}
func (e *Event) GetAttach() string {
	return e.attach
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

// Validate the alarm and add it to the event
func (e *Event) AddAlarm(alarm Alarm) error {
	if err := alarm.Validate(); err != nil {
		return err
	}
	e.hasModified()
	if e.alarm == nil {
		e.alarm = make([]Alarm, 0)
	}
	e.alarm = append(e.alarm, alarm)
	return nil
}
func (e *Event) RemoveAlarm(alarmUID string) error {
	if e.alarm == nil {
		return fmt.Errorf("alarm is empty")
	}
	e.hasModified()
	for i, a := range e.alarm {
		if a.uid == alarmUID {
			e.alarm = append(e.alarm[:i], e.alarm[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("alarm not found")
}
func (e *Event) SetAttachment(attach string) {
	e.hasModified()
	e.attach = attach
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

	var sb strings.Builder

	if _, err := sb.WriteString("BEGIN:VEVENT\n"); err != nil {
		return "", err
	}
	if _, err := sb.WriteString("UID:" + e.id + "\n"); err != nil {
		return "", err
	}
	if _, err := sb.WriteString("SUMMARY:" + e.summary + "\n"); err != nil {
		return "", err
	}

	if e.description != "" {
		if _, err := sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\n", e.description)); err != nil {
			return "", err
		}
	}
	if e.location != "" {
		if _, err := sb.WriteString(fmt.Sprintf("LOCATION:%s\n", e.location)); err != nil {
			return "", err
		}
	}
	if e.url != "" {
		if _, err := sb.WriteString(fmt.Sprintf("URL:%s\n", e.url)); err != nil {
			return "", err
		}
	}

	startDateStr, err := timeToStr(e.startDate)
	if err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("DTSTART:%s\n", startDateStr)); err != nil {
		return "", err
	}
	endDateStr, err := timeToStr(e.endDate)
	if err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("DTEND:%s\n", endDateStr)); err != nil {
		return "", err
	}

	if len(e.attendee) > 0 {
		for _, attendee := range e.attendee {
			attendeeStr, err := attendee.Marshal()
			if err != nil {
				return "", err
			}
			if _, err := sb.WriteString(attendeeStr); err != nil {
				return "", err
			}
		}
	}
	if e.organizer != "" {
		if _, err := sb.WriteString(fmt.Sprintf("ORGANIZER:%s\n", e.organizer)); err != nil {
			return "", err
		}
	}

	if _, err := sb.WriteString(fmt.Sprintf("CREATED:%s\n", e.createdAt.Format("20060102T150405Z"))); err != nil {
		return "", err
	}
	if !e.updatedAt.IsZero() {
		if _, err := sb.WriteString(fmt.Sprintf("LAST-MODIFIED:%s\n", e.updatedAt.Format("20060102T150405Z"))); err != nil {
			return "", err
		}
	}

	if e.sequence >= 0 {
		if _, err := sb.WriteString(fmt.Sprintf("SEQUENCE:%d\n", e.sequence)); err != nil {
			return "", err
		}
	}
	for _, alarm := range e.alarm {
		alarmStr, err := alarm.Marshal()
		if err != nil {
			return "", err
		}
		if _, err := sb.WriteString(alarmStr); err != nil {
			return "", err
		}
	}

	if e.rrule != nil {
		if _, err := sb.WriteString(fmt.Sprintf("RRULE:%s\n", e.rrule.String())); err != nil {
			return "", err
		}
	}
	for _, exdate := range e.exdate {
		exDateStr, err := timeToStr(exdate)
		if err != nil {
			return "", err
		}
		if _, err := sb.WriteString(fmt.Sprintf("EXDATE:%s\n", exDateStr)); err != nil {
			return "", err
		}
	}
	for _, rdate := range e.rdate {
		rdateStr, err := timeToStr(rdate)
		if err != nil {
			return "", nil
		}
		if _, err := sb.WriteString(fmt.Sprintf("RDATE:%s\n", rdateStr)); err != nil {
			return "", err
		}
	}

	if !e.recurrenceID.IsZero() {
		recurrenceIDStr, err := timeToStr(e.recurrenceID)
		if err != nil {
			return "", err
		}
		if _, err := sb.WriteString(fmt.Sprintf("RECURRENCE-ID:%s\n", recurrenceIDStr)); err != nil {
			return "", err
		}
	}

	if _, err := sb.WriteString(fmt.Sprintf("DTSTAMP:%s\n", time.Now().Format("20060102T150405Z"))); err != nil {
		return "", err
	}
	if _, err := sb.WriteString("END:VEVENT\n"); err != nil {
		return "", err
	}

	return sb.String(), nil
}
