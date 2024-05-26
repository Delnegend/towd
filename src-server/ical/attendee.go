package ical

import (
	"fmt"
	"log/slog"
	"strings"
)

type AttendeeRole string

var (
	AttendeeRoleChair AttendeeRole = "CHAIR"           // organizer
	AttendeeRoleReq   AttendeeRole = "REQ-PARTICIPANT" // required participant
	AttendeeRoleOpt   AttendeeRole = "OPT-PARTICIPANT" // optional participant
	AttendeeRoleNon   AttendeeRole = "NON-PARTICIPANT" // for information only
)

type AttendeeCutype string

var (
	AttendeeCutypeIndividual AttendeeCutype = "INDIVIDUAL" // individual
	AttendeeCutypeGroup      AttendeeCutype = "GROUP"      // group
	AttendeeCutypeResource   AttendeeCutype = "RESOURCE"   // resource
	AttendeeCutypeRoom       AttendeeCutype = "ROOM"       // room
	AttendeeCutypeUnknown    AttendeeCutype = "UNKNOWN"    // unknown
)

type AttendeeCalAdrr string

type Attendee struct {
	// Common name
	cn   AttendeeCalAdrr
	role AttendeeRole
	// Répondez s'il vous plaît, French for "Please respond"
	rsvp bool
	// Calendar user type
	cuType        AttendeeCutype
	member        []AttendeeCalAdrr
	delegatedTo   []AttendeeCalAdrr
	delegatedFrom []AttendeeCalAdrr
	sentBy        AttendeeCalAdrr
	// points to the directory information corresponding to the attendee.
	dir string
}

// #region Getters
func (a *Attendee) GetCN() AttendeeCalAdrr {
	return a.cn
}

func (a *Attendee) GetRole() AttendeeRole {
	return a.role
}

func (a *Attendee) GetRSVP() bool {
	return a.rsvp
}

func (a *Attendee) GetCUType() AttendeeCutype {
	return a.cuType
}

func (a *Attendee) GetMember() []AttendeeCalAdrr {
	return a.member
}

func (a *Attendee) GetDelegatedTo() []AttendeeCalAdrr {
	return a.delegatedTo
}

func (a *Attendee) GetDelegatedFrom() []AttendeeCalAdrr {
	return a.delegatedFrom
}

func (a *Attendee) GetSentBy() AttendeeCalAdrr {
	return a.sentBy
}

func (a *Attendee) GetDir() string {
	return a.dir
}

// #endregion

// #region Setters
func (a *Attendee) SetCN(cn AttendeeCalAdrr) {
	a.cn = cn
}

func (a *Attendee) SetRole(role AttendeeRole) {
	a.role = role
}

func (a *Attendee) SetRSVP(rsvp bool) {
	a.rsvp = rsvp
}

func (a *Attendee) SetCUType(cuType AttendeeCutype) {
	a.cuType = cuType
}

func (a *Attendee) SetMember(member []AttendeeCalAdrr) {
	a.member = member
}

func (a *Attendee) SetDelegatedTo(delegatedTo []AttendeeCalAdrr) {
	a.delegatedTo = delegatedTo
}

func (a *Attendee) SetDelegatedFrom(delegatedFrom []AttendeeCalAdrr) {
	a.delegatedFrom = delegatedFrom
}

func (a *Attendee) SetSentBy(sentBy AttendeeCalAdrr) {
	a.sentBy = sentBy
}

func (a *Attendee) SetDir(dir string) {
	a.dir = dir
}

// #endregion

func (a *Attendee) Validate() error {
	if a.cn == "" {
		return fmt.Errorf("CN is required")
	}
	return nil
}
func (a *Attendee) Marshal() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	var sb strings.Builder
	if _, err := sb.WriteString("ATTENDEE;"); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("CN=%s;", a.cn)); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("ROLE=%s;", a.role)); err != nil {
		return "", err
	}
	if a.rsvp {
		if _, err := sb.WriteString("RSVP=TRUE;"); err != nil {
			return "", err
		}
	}
	if _, err := sb.WriteString(fmt.Sprintf("CUTYPE=%s;", a.cuType)); err != nil {
		return "", err
	}
	for _, m := range a.member {
		if _, err := sb.WriteString(fmt.Sprintf("MEMBER=%s;", m)); err != nil {
			return "", err
		}
	}
	for _, d := range a.delegatedTo {
		if _, err := sb.WriteString(fmt.Sprintf("DELEGATED-TO=%s;", d)); err != nil {
			return "", err
		}
	}
	for _, d := range a.delegatedFrom {
		if _, err := sb.WriteString(fmt.Sprintf("DELEGATED-FROM=%s;", d)); err != nil {
			return "", err
		}
	}
	if a.sentBy != "" {
		if _, err := sb.WriteString(fmt.Sprintf("SENT-BY=%s;", a.sentBy)); err != nil {
			return "", err
		}
	}
	if a.dir != "" {
		if _, err := sb.WriteString(fmt.Sprintf("DIR=%s;", a.dir)); err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}
func (a *Attendee) Unmarshal(data string) error {
	// split by ;
	data = strings.TrimPrefix(data, "ATTENDEE;")
	slice := strings.Split(data, ";")
	for _, s := range slice {
		// split by =
		parts := strings.Split(s, "=")
		if len(parts) < 2 {
			return fmt.Errorf("expected key=value, got %s", s)
		}
		key, value := parts[0], parts[1]
		switch key {
		case "CN":
			if value == "" {
				return fmt.Errorf("CN is required")
			}
			a.cn = AttendeeCalAdrr(value)
		case "ROLE":
			a.role = AttendeeRole(value)
		case "RSVP":
			a.rsvp = value == "TRUE"
		case "CUTYPE":
			a.cuType = AttendeeCutype(value)
		case "MEMBER":
			a.member = append(a.member, AttendeeCalAdrr(value))
		case "DELEGATED-TO":
			a.delegatedTo = append(a.delegatedTo, AttendeeCalAdrr(value))
		case "DELEGATED-FROM":
			a.delegatedFrom = append(a.delegatedFrom, AttendeeCalAdrr(value))
		case "SENT-BY":
			a.sentBy = AttendeeCalAdrr(value)
		case "DIR":
			a.dir = value
		default:
			slog.Warn("unhandled key", "key", key, "value", value)
		}
	}
	return nil
}
