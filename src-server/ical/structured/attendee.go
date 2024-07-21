package structured

import (
	"fmt"
	"strings"
)

type (
	AttendeeCustomertype      string
	AttendeeRole              string
	AttendeeParticipantStatus string
	AttendeeCommonName        string
)

var (
	AttendeeCutypeIndividual AttendeeCustomertype = "INDIVIDUAL"
	AttendeeCutypeGroup      AttendeeCustomertype = "GROUP"
	AttendeeCutypeResource   AttendeeCustomertype = "RESOURCE"
	AttendeeCutypeRoom       AttendeeCustomertype = "ROOM"
	AttendeeCutypeUnknown    AttendeeCustomertype = "UNKNOWN"

	AttendeeRoleChair AttendeeRole = "CHAIR"           // organizer
	AttendeeRoleReq   AttendeeRole = "REQ-PARTICIPANT" // required participant
	AttendeeRoleOpt   AttendeeRole = "OPT-PARTICIPANT" // optional participant
	AttendeeRoleNon   AttendeeRole = "NON-PARTICIPANT" // for information only

	AttendeePartStatNeedsAction AttendeeParticipantStatus = "NEEDS-ACTION"
	AttendeePartStatAccepted    AttendeeParticipantStatus = "ACCEPTED"
	AttendeePartStatDeclined    AttendeeParticipantStatus = "DECLINED"
	AttendeePartStatTentative   AttendeeParticipantStatus = "TENTATIVE"
	AttendeePartStatCancelled   AttendeeParticipantStatus = "CANCELLED"
	AttendeePartStatXName       AttendeeParticipantStatus = "X-NAME"
)

type Attendee struct {
	cuType   AttendeeCustomertype // Calendar user type
	role     AttendeeRole
	partStat AttendeeParticipantStatus
	cn       AttendeeCommonName

	member        []AttendeeCommonName
	delegatedTo   []AttendeeCommonName
	delegatedFrom []AttendeeCommonName
	rsvp          bool // Répondez s'il vous plaît
	sentBy        AttendeeCommonName

	customProperties []string
}

func NewAttendee() Attendee {
	return Attendee{}
}

// Set the attendee CUType
func (a *Attendee) SetCuType(cuType AttendeeCustomertype) *Attendee {
	a.cuType = cuType
	return a
}

// Set the attendee role
func (a *Attendee) SetRole(role AttendeeRole) *Attendee {
	a.role = role
	return a
}

// Set the attendee partStat
func (a *Attendee) SetPartStat(partStat AttendeeParticipantStatus) *Attendee {
	a.partStat = partStat
	return a
}

// Set the attendee CN
func (a *Attendee) SetCn(cn AttendeeCommonName) *Attendee {
	a.cn = cn
	return a
}

// Set the attendee MEMBER
func (a *Attendee) AddMember(member AttendeeCommonName) *Attendee {
	a.member = append(a.member, member)
	return a
}

// Set the attendee DELEGATED-TO
func (a *Attendee) AddDelegatedTo(delegatedTo AttendeeCommonName) *Attendee {
	a.delegatedTo = append(a.delegatedTo, delegatedTo)
	return a
}

// Set the attendee DELEGATED-FROM
func (a *Attendee) AddDelegatedFrom(delegatedFrom AttendeeCommonName) *Attendee {
	a.delegatedFrom = append(a.delegatedFrom, delegatedFrom)
	return a
}

// Set the attendee RSVP
func (a *Attendee) SetRsvp(rsvp bool) *Attendee {
	a.rsvp = rsvp
	return a
}

// Set the attendee SENT-BY
func (a *Attendee) SetSentBy(sentBy AttendeeCommonName) *Attendee {
	a.sentBy = sentBy
	return a
}

func (a *Attendee) validate() error {
	switch {
	case a.cuType == "":
		return fmt.Errorf("CUTYPE is required")
	case a.role == "":
		return fmt.Errorf("ROLE is required")
	case a.partStat == "":
		return fmt.Errorf("PARTSTAT is required")
	case a.cn == "":
		return fmt.Errorf("CN is required")
	default:
		return nil
	}
}

// Convert the attendee into an iCalendar string. This method is intended to be
// used internally only. Example usage:
//
//	var sb strings.Builder
//
//	if err := structured.NewAttendee().
//	    SetCuType(structured.AttendeeCutypeIndividual).
//	    SetRole(structured.AttendeeRoleReq).
//	    SetCn(structured.NewCommonName("Attendee Name", "attendee@example.com")).
//	    AddMember(structured.NewCommonName("Member Name", "member@example.com")).
//	    AddDelegatedTo(structured.NewCommonName("Delegated To Name", "delegated@example.com")).
//	    AddDelegatedFrom(structured.NewCommonName("Delegated From Name", "delegated@example.com")).
//	    SetRsvp(true).
//	    SetSentBy(structured.NewCommonName("Sent By Name", "sent@example.com")).
//	    ToIcal(&sb); err != nil {
//	    log.Fatal(err)
//	}
func (a *Attendee) ToIcal(writer func(string) (int, error)) error {
	if err := a.validate(); err != nil {
		return err
	}

	writer(fmt.Sprintf("ATTENDEE;CN=%s", a.cn))
	if a.cuType != "" {
		writer(fmt.Sprintf(";CUTYPE=%s", a.cuType))
	}
	if a.role != "" {
		writer(fmt.Sprintf(";ROLE=%s", a.role))
	}
	if a.partStat != "" {
		writer(fmt.Sprintf(";PARTSTAT=%s", a.partStat))
	}
	if a.rsvp {
		writer(";RSVP=TRUE")
	}
	for _, m := range a.member {
		writer(fmt.Sprintf(";MEMBER=%s", m))
	}
	for _, d := range a.delegatedTo {
		writer(fmt.Sprintf(";DELEGATED-TO=%s", d))
	}
	for _, d := range a.delegatedFrom {
		writer(fmt.Sprintf(";DELEGATED-FROM=%s", d))
	}
	if a.sentBy != "" {
		writer(fmt.Sprintf(";SENT-BY=%s", a.sentBy))
	}
	writer("\n")
	return nil
}

// Parse an iCalendar string into an Attendee{} struct. Example usage:
//
//	attendee, err := structured.NewAttendee().FromIcal("ATTENDEE;CN=Attendee Name:mailto:attendee@example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (a *Attendee) FromIcal(data string) error {
	data = strings.TrimPrefix(data, "ATTENDEE;")
	slice := strings.Split(data, ";")
	for _, s := range slice {
		// split by =
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			a.customProperties = append(a.customProperties, s)
			continue
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
		default:
			a.customProperties = append(a.customProperties, fmt.Sprintf("%s=%s", key, value))
		}
	}
	if err := a.validate(); err != nil {
		return err
	}
	return nil
}
