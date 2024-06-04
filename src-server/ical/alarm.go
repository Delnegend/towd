package ical

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type (
	AlarmAction string
)

var (
	AlarmActionAudio     AlarmAction = "AUDIO"
	AlarmActionDisplay   AlarmAction = "DISPLAY"
	AlarmActionEmail     AlarmAction = "EMAIL"
	AlarmActionProcedure AlarmAction = "PROCEDURE"
)

type Alarm struct {
	uid         string
	action      AlarmAction
	trigger     string
	duration    string
	repeat      int
	attach      string
	description string
	summary     string
	attendee    []Attendee
}

func NewAlarm() Alarm {
	return Alarm{
		uid:      uuid.New().String(),
		attendee: make([]Attendee, 0),
	}
}

// #region Getters
func (a *Alarm) GetAction() AlarmAction {
	return a.action
}
func (a *Alarm) GetTrigger() string {
	return a.trigger
}
func (a *Alarm) GetDuration() string {
	return a.duration
}
func (a *Alarm) GetRepeat() int {
	return a.repeat
}
func (a *Alarm) GetAttachment() string {
	return a.attach
}
func (a *Alarm) GetDescription() string {
	return a.description
}
func (a *Alarm) GetSummary() string {
	return a.summary
}
func (a *Alarm) GetAttendee() []Attendee {
	return a.attendee
}

// #endregion

// #region Setters
func (a *Alarm) SetAction(action AlarmAction) {
	a.action = action
}
func (a *Alarm) SetTrigger(trigger string) error {
	rgx, err := regexp.Compile(`^(-PT)(\d+)(M|H)$`)
	if err != nil {
		return err
	}
	if !rgx.MatchString(trigger) {
		return fmt.Errorf(errWrongAlarmTriggerFormat)
	}
	a.trigger = trigger
	return nil
}
func (a *Alarm) SetDuration(duration string) error {
	rgx, err := regexp.Compile(`^(PT)(\d+)(M|H)$`)
	if err != nil {
		return err
	}
	if !rgx.MatchString(duration) {
		return fmt.Errorf(errWrongAlarmDurationFormat)
	}
	a.duration = duration
	return nil
}
func (a *Alarm) SetRepeat(repeat int) error {
	if repeat < 0 {
		return fmt.Errorf("repeat must be positive")
	}
	a.repeat = repeat
	return nil
}
func (a *Alarm) SetAttachment(attachment string) {
	a.attach = attachment
}
func (a *Alarm) SetDescription(description string) {
	a.description = description
}
func (a *Alarm) SetSummary(summary string) {
	a.summary = summary
}
func (a *Alarm) AddAttendee(attendee Attendee) {
	if a.attendee == nil {
		a.attendee = make([]Attendee, 0)
	}
	a.attendee = append(a.attendee, attendee)
}
func (a *Alarm) RemoveAttendee(attendeeCn AttendeeCommonName) error {
	if a.attendee == nil {
		return fmt.Errorf("attendee is empty")
	}
	for i, attendee := range a.attendee {
		if attendee.cn == attendeeCn {
			a.attendee = append(a.attendee[:i], a.attendee[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("attendee not found")
}

// #endregion

func (a *Alarm) Validate() error {
	switch {
	case a.uid == "":
		return fmt.Errorf("uid is required")
	case a.action == "":
		return fmt.Errorf("action is required")
	case a.trigger == "":
		return fmt.Errorf("trigger is required")
	case a.duration != "" && a.repeat == 0:
		return fmt.Errorf("repeat is required")
	case a.duration == "" && a.repeat != 0:
		return fmt.Errorf("duration is required")
	}

	switch a.action {
	case AlarmActionEmail:
		switch {
		case a.description == "":
			return fmt.Errorf("description is required for email action")
		case a.summary == "":
			return fmt.Errorf("summary is required for email action")
		case len(a.attendee) == 0:
			return fmt.Errorf("attendee is required for email action")
		}

	case AlarmActionDisplay:
		if a.description == "" {
			return fmt.Errorf("description is required for display action")
		}
	}

	return nil
}

func (a *Alarm) Marshal() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("BEGIN:VALARM\nUID:%s\nACTION:%s\nTRIGGER;VALUE=DATE-TIME:%s\n", a.uid, a.action, a.trigger))
	if a.duration != "" {
		sb.WriteString(fmt.Sprintf("DURATION:%s\n", a.duration))
	}
	if a.repeat != 0 {
		sb.WriteString(fmt.Sprintf("REPEAT:%d\n", a.repeat))
	}
	if a.attach != "" {
		sb.WriteString(fmt.Sprintf("ATTACH;%s\n", a.attach))
	}
	if a.description != "" {
		sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\n", a.description))
	}
	if a.summary != "" {
		sb.WriteString(fmt.Sprintf("SUMMARY:%s\n", a.summary))
	}
	for _, attendee := range a.attendee {
		attendeeStr, err := attendee.Marshal()
		if err != nil {
			return "", err
		}
		sb.WriteString(attendeeStr)
	}
	sb.WriteString("END:VALARM\n")

	return sb.String(), nil
}
