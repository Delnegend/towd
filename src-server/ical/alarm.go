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
	if _, err := sb.WriteString("BEGIN:VALARM\n"); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("UID:%s\n", a.uid)); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("ACTION:%s\n", a.action)); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("TRIGGER;VALUE=DATE-TIME:%s\n", a.trigger)); err != nil {
		return "", err
	}
	if a.duration != "" {
		if _, err := sb.WriteString(fmt.Sprintf("DURATION:%s\n", a.duration)); err != nil {
			return "", err
		}
	}
	if a.repeat != 0 {
		if _, err := sb.WriteString(fmt.Sprintf("REPEAT:%d\n", a.repeat)); err != nil {
			return "", err
		}
	}
	if a.attach != "" {
		if _, err := sb.WriteString(fmt.Sprintf("ATTACH;%s\n", a.attach)); err != nil {
			return "", err
		}
	}
	if a.description != "" {
		if _, err := sb.WriteString("DESCRIPTION:" + a.description + "\n"); err != nil {
			return "", err
		}
	}
	if a.summary != "" {
		if _, err := sb.WriteString("SUMMARY:" + a.summary + "\n"); err != nil {
			return "", err
		}
	}
	for _, attendee := range a.attendee {
		attendeeStr, err := attendee.Marshal()
		if err != nil {
			return "", err
		}
		if _, err := sb.WriteString(attendeeStr); err != nil {
			return "", err
		}
	}
	if _, err := sb.WriteString("END:VALARM\n"); err != nil {
		return "", err
	}

	return sb.String(), nil
}
