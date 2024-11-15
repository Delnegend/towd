package structured

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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

	CustomProperties []string
}

func NewAlarm() Alarm {
	return Alarm{
		uid: uuid.New().String(),
	}
}

// Set the alarm UID
func (a *Alarm) SetUid(uid string) *Alarm {
	a.uid = uid
	return a
}

// Set the alarm action
func (a *Alarm) SetAction(action AlarmAction) *Alarm {
	a.action = action
	return a
}

// Set the alarm trigger
func (a *Alarm) SetTrigger(trigger string) *Alarm {
	a.trigger = trigger
	return a
}

// Set the alarm duration
func (a *Alarm) SetDuration(duration string) *Alarm {
	a.duration = duration
	return a
}

// Set the alarm repeat
func (a *Alarm) SetRepeat(repeat int) *Alarm {
	a.repeat = repeat
	return a
}

// Set the alarm attach
func (a *Alarm) SetAttach(attach string) *Alarm {
	a.attach = attach
	return a
}

// Set the alarm description
func (a *Alarm) SetDescription(description string) *Alarm {
	a.description = description
	return a
}

// Set the alarm summary
func (a *Alarm) SetSummary(summary string) *Alarm {
	a.summary = summary
	return a
}

// Set the alarm attendee
func (a *Alarm) SetAttendee(attendee []Attendee) *Alarm {
	a.attendee = attendee
	return a
}

// Add a custom property to the alarm. This method is intended to be used
// internally only.
func (a *Alarm) AddCustomProperty(property string) *Alarm {
	a.CustomProperties = append(a.CustomProperties, property)
	return a
}

func (a *Alarm) validate() error {
	triggerRgx := regexp.MustCompile(`^(-PT)(\d+)(M|H)$`)
	durationRgx := regexp.MustCompile(`^(PT)(\d+)(M|H)$`)

	switch {
	case a.uid == "":
		return fmt.Errorf("UID is required")
	case a.action == "":
		return fmt.Errorf("action is required")
	case a.trigger == "":
		return fmt.Errorf("trigger is required")
	case a.duration != "" && a.repeat == 0:
		return fmt.Errorf("repeat is required when duration is set")
	case a.duration == "" && a.repeat != 0:
		return fmt.Errorf("duration is required when repeat is set")
	case (a.trigger != "") && !triggerRgx.MatchString(a.trigger):
		return fmt.Errorf("trigger format is invalid")
	case (a.duration != "") && !durationRgx.MatchString(a.duration):
		return fmt.Errorf("duration format is invalid")
	}
	return nil
}

// Add an iCalendar property to the alarm.
// Unhandled properties will be stored in the CustomProperties array.
func (a *Alarm) AddIcalProperty(property string) {
	slice := strings.SplitN(property, ":", 2)
	if len(slice) != 2 {
		a.CustomProperties = append(a.CustomProperties, property)
		return
	}

	key := strings.ToUpper(strings.TrimSpace(slice[0]))
	value := strings.TrimSpace(slice[1])

	switch key {
	case "UID":
		a.uid = value
	case "ACTION":
		a.action = AlarmAction(value)
	case "ATTACH":
		a.attach = value
	case "DESCRIPTION":
		a.description = value
	case "DURATION":
		a.duration = value
	case "REPEAT":
		if repeat, err := strconv.Atoi(value); err == nil {
			a.repeat = repeat
		} else {
			a.repeat = 0
		}
	case "SUMMARY":
		a.summary = value
	case "TRIGGER":
		a.trigger = value
	default:
		a.CustomProperties = append(a.CustomProperties, property)
	}
}

// Convert the alarm into an iCalendar string. This method is intended to be used
// internally only. Example usage:
//
//	var sb strings.Builder
//
//	if err := structured.NewAlarm().
//	    SetAction(structured.AlarmActionAudio).
//	    SetTrigger("20220101T000000Z").
//	    SetDuration("PT1H").
//	    SetRepeat(1).
//	    SetSummary("Alarm summary").
//	    AddAttendee(structured.NewAttendee().
//	        SetCuType(structured.AttendeeCutypeIndividual).
//	        SetRole(structured.AttendeeRoleReq).
//	        SetCn(structured.NewCommonName("Alarm Name", "alarm@example.com"))).
//	    AddCustomProperty("X-MY-CUSTOM-PROPERTY:value").
//	    ToIcal(&sb); err != nil {
//	    log.Fatal(err)
//	}
func (a *Alarm) ToIcal(writer func(string)) {
	if err := a.validate(); err != nil {
		slog.Warn("Alarm.ToIcal", "err", err)
		return
	}

	writer("BEGIN:VALARM\n")
	writer("UID:" + a.uid + "\n")
	writer("ACTION:" + string(a.action) + "\n")
	if a.trigger != "" {
		writer("TRIGGER;VALUE=DATE-TIME:" + a.trigger + "\n")
	}
	if a.duration != "" {
		writer("DURATION:" + a.duration + "\n")
	}
	if a.repeat != 0 {
		writer("REPEAT:" + strconv.Itoa(a.repeat) + "\n")
	}
	if a.attach != "" {
		writer("ATTACH;" + a.attach + "\n")
	}
	if a.description != "" {
		writer("DESCRIPTION:" + a.description + "\n")
	}
	if a.summary != "" {
		writer("SUMMARY:" + a.summary + "\n")
	}
	for _, attendee := range a.attendee {
		attendee.ToIcal(writer)
	}
	writer("END:VALARM\n")
}
