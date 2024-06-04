package ical

import (
	"fmt"
	"log/slog"
	"strings"
)

type (
	AttendeeRole              string
	AttendeeCustomertype      string
	AttendeeParticipantStatus string
	AttendeeCommonName        string
)

var (
	AttendeeRoleChair AttendeeRole = "CHAIR"           // organizer
	AttendeeRoleReq   AttendeeRole = "REQ-PARTICIPANT" // required participant
	AttendeeRoleOpt   AttendeeRole = "OPT-PARTICIPANT" // optional participant
	AttendeeRoleNon   AttendeeRole = "NON-PARTICIPANT" // for information only

	AttendeeCutypeIndividual AttendeeCustomertype = "INDIVIDUAL"
	AttendeeCutypeGroup      AttendeeCustomertype = "GROUP"
	AttendeeCutypeResource   AttendeeCustomertype = "RESOURCE"
	AttendeeCutypeRoom       AttendeeCustomertype = "ROOM"
	AttendeeCutypeUnknown    AttendeeCustomertype = "UNKNOWN"

	AttendeePartStatNeedsAction AttendeeParticipantStatus = "NEEDS-ACTION"
	AttendeePartStatAccepted    AttendeeParticipantStatus = "ACCEPTED"
	AttendeePartStatDeclined    AttendeeParticipantStatus = "DECLINED"
	AttendeePartStatTentative   AttendeeParticipantStatus = "TENTATIVE"
	AttendeePartStatCancelled   AttendeeParticipantStatus = "CANCELLED"
	AttendeePartStatXName       AttendeeParticipantStatus = "X-NAME"
)

type Attendee struct {
	// Common name
	cn   AttendeeCommonName
	role AttendeeRole
	// Répondez s'il vous plaît, French for "Please respond"
	rsvp bool
	// Calendar user type
	cuType        AttendeeCustomertype
	partStat      AttendeeParticipantStatus
	member        []AttendeeCommonName
	delegatedTo   []AttendeeCommonName
	delegatedFrom []AttendeeCommonName
	sentBy        AttendeeCommonName
	// points to the directory information corresponding to the attendee.
	dir string
}

// #region Getters
func (a *Attendee) GetCN() AttendeeCommonName {
	return a.cn
}

func (a *Attendee) GetRole() AttendeeRole {
	return a.role
}

func (a *Attendee) GetRSVP() bool {
	return a.rsvp
}

func (a *Attendee) GetCUType() AttendeeCustomertype {
	return a.cuType
}

func (a *Attendee) GetPartStat() AttendeeParticipantStatus {
	return a.partStat
}
func (a *Attendee) GetMember() []AttendeeCommonName {
	return a.member
}

func (a *Attendee) GetDelegatedTo() []AttendeeCommonName {
	return a.delegatedTo
}

func (a *Attendee) GetDelegatedFrom() []AttendeeCommonName {
	return a.delegatedFrom
}

func (a *Attendee) GetSentBy() AttendeeCommonName {
	return a.sentBy
}

func (a *Attendee) GetDir() string {
	return a.dir
}

// #endregion

// #region Setters
func (a *Attendee) SetCN(cn AttendeeCommonName) {
	a.cn = cn
}

func (a *Attendee) SetRole(role AttendeeRole) {
	a.role = role
}

func (a *Attendee) SetRSVP(rsvp bool) {
	a.rsvp = rsvp
}

func (a *Attendee) SetCUType(cuType AttendeeCustomertype) {
	a.cuType = cuType
}

func (a *Attendee) SetPartStat(partStat AttendeeParticipantStatus) {
	a.partStat = partStat
}
func (a *Attendee) SetMember(member []AttendeeCommonName) {
	a.member = member
}

func (a *Attendee) SetDelegatedTo(delegatedTo []AttendeeCommonName) {
	a.delegatedTo = delegatedTo
}

func (a *Attendee) SetDelegatedFrom(delegatedFrom []AttendeeCommonName) {
	a.delegatedFrom = delegatedFrom
}

func (a *Attendee) SetSentBy(sentBy AttendeeCommonName) {
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
	sb.WriteString(fmt.Sprintf("ATTENDEE;CN=%s;ROLE=%s", a.cn, a.role))
	if a.rsvp {
		sb.WriteString("RSVP=TRUE")
	}
	sb.WriteString(fmt.Sprintf(";PARTSTAT=%s", a.partStat))
	for _, m := range a.member {
		sb.WriteString(fmt.Sprintf(";MEMBER=%s", m))
	}
	for _, d := range a.delegatedTo {
		sb.WriteString(fmt.Sprintf(";DELEGATED-TO=%s", d))
	}
	for _, d := range a.delegatedFrom {
		sb.WriteString(fmt.Sprintf(";DELEGATED-FROM=%s", d))
	}
	if a.sentBy != "" {
		sb.WriteString(fmt.Sprintf(";SENT-BY=%s", a.sentBy))
	}
	if a.dir != "" {
		sb.WriteString(fmt.Sprintf(";DIR=%s", a.dir))
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
			a.cn = AttendeeCommonName(value)
		case "ROLE":
			switch value {
			case "CHAIR":
				a.role = AttendeeRoleChair
			case "REQ-PARTICIPANT":
				a.role = AttendeeRoleReq
			case "OPT-PARTICIPANT":
				a.role = AttendeeRoleOpt
			case "NON-PARTICIPANT":
				a.role = AttendeeRoleNon
			default:
				return fmt.Errorf("invalid role: %s", value)
			}
		case "RSVP":
			a.rsvp = value == "TRUE"
		case "CUTYPE":
			switch value {
			case "INDIVIDUAL":
				a.cuType = AttendeeCutypeIndividual
			case "GROUP":
				a.cuType = AttendeeCutypeGroup
			case "RESOURCE":
				a.cuType = AttendeeCutypeResource
			case "ROOM":
				a.cuType = AttendeeCutypeRoom
			case "UNKNOWN":
				a.cuType = AttendeeCutypeUnknown
			default:
				return fmt.Errorf("invalid CUTYPE: %s", value)
			}
		case "PARTSTAT":
			switch value {
			case "NEEDS-ACTION":
				a.partStat = AttendeePartStatNeedsAction
			case "ACCEPTED":
				a.partStat = AttendeePartStatAccepted
			case "DECLINED":
				a.partStat = AttendeePartStatDeclined
			case "TENTATIVE":
				a.partStat = AttendeePartStatTentative
			case "CANCELLED":
				a.partStat = AttendeePartStatCancelled
			case "X-NAME":
				a.partStat = AttendeePartStatXName
			default:
				return fmt.Errorf("invalid PARTSTAT: %s", value)
			}
		case "MEMBER":
			a.member = append(a.member, AttendeeCommonName(value))
		case "DELEGATED-TO":
			a.delegatedTo = append(a.delegatedTo, AttendeeCommonName(value))
		case "DELEGATED-FROM":
			a.delegatedFrom = append(a.delegatedFrom, AttendeeCommonName(value))
		case "SENT-BY":
			a.sentBy = AttendeeCommonName(value)
		case "DIR":
			a.dir = value
		default:
			slog.Warn("unhandled key", "key", key, "value", value)
		}
	}
	return nil
}
