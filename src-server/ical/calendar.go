package ical

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xyedo/rrule"
)

type Calendar struct {
	id          string
	prodId      string
	name        string
	description string
	url         string
	events      []Event
}

func NewCalendar() Calendar {
	return Calendar{
		id: uuid.NewString(),
	}
}

func UnmarshalFile(path string) (*Calendar, *slogError) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &slogError{
			Msg:  "error opening file",
			Args: []interface{}{"path", path, "err", err},
		}
	}
	defer file.Close()

	lineCh := make(chan string)

	go func() {
		defer close(lineCh)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	return unmarshalCh(lineCh)
}

func UnmarshalUrl(url_ string) (*Calendar, *slogError) {
	validUrl, err := url.ParseRequestURI(url_)
	if err != nil {
		return nil, &slogError{Msg: err.Error()}
	}

	req, err := http.NewRequest("GET", validUrl.String(), nil)
	if err != nil {
		return nil, &slogError{Msg: err.Error()}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &slogError{Msg: err.Error()}
	}
	defer resp.Body.Close()

	lineCh := make(chan string)

	go func() {
		defer close(lineCh)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	return unmarshalCh(lineCh)
}

func unmarshalCh(lineCh chan string) (*Calendar, *slogError) {
	cal := NewCalendar()
	var mode string
	lineCount := -1
	eventCount := 0

	// "lookahead" to merge lines that are split
	mergedLineCh := make(chan string)
	go func() {
		defer close(mergedLineCh)

		var lastLine string
		for currentLine := range lineCh {
			switch strings.HasPrefix(currentLine, " ") {
			case true:
				currentLine = lastLine + strings.TrimPrefix(currentLine, " ")
			case false:
				if lastLine != "" {
					mergedLineCh <- lastLine
				}
			}
			lineCount++
			lastLine = currentLine
		}
	}()

	newEvent := NewEvent()
	newAlarm := NewAlarm()

	for line := range mergedLineCh {
		switch {
		case strings.HasPrefix(line, "ATTACH"):
			val := strings.TrimPrefix(line, "ATTACH;")
			switch mode {
			case "event":
				newEvent.SetAttachment(val)
				continue
			case "alarm":
				newAlarm.SetAttachment(val)
				continue
			}
			continue
		case strings.HasPrefix(line, "ATTENDEE"):
			attendee := Attendee{}
			if err := attendee.Unmarshal(line); err != nil {
				return nil, &slogError{
					Msg:  "can't unmarshal attendee",
					Args: []interface{}{"line", lineCount, "content", line, "err", err},
				}
			}
			switch mode {
			case "event":
				newEvent.AddAttendee(attendee)
			case "alarm":
				newAlarm.AddAttendee(attendee)
			}
			continue
		case strings.HasPrefix(line, "ORGANIZER"):
			newEvent.SetOrganizer(strings.TrimPrefix(line, "ORGANIZER;"))
			continue
		}

		slice := strings.SplitN(line, ":", 2)
		if len(slice) != 2 {
			return nil, &slogError{
				Msg:  "invalid line format",
				Args: []interface{}{"line", lineCount, "content", line},
			}
		}
		key := slice[0]
		value := slice[1]

		switch key {
		case "BEGIN":
			switch value {
			case "VCALENDAR":
				if mode == "calendar" {
					return nil, errNestedBlock("VCALENDAR", lineCount, line)
				}
				mode = "calendar"
			case "VTIMEZONE":
				if mode == "timezone" {
					return nil, errNestedBlock("VTIMEZONE", lineCount, line)
				}
				mode = "timezone"
			case "STANDARD":
				if mode == "standard" {
					return nil, errNestedBlock("STANDARD", lineCount, line)
				}
				if mode != "timezone" {
					return nil, &slogError{
						Msg:  "STANDARD block not in VTIMEZONE block",
						Args: []interface{}{"line", lineCount, "content", line},
					}
				}
				mode = "standard"
			case "DAYLIGHT":
				if mode == "daylight" {
					return nil, errNestedBlock("DAYLIGHT", lineCount, line)
				}
				if mode != "timezone" {
					return nil, &slogError{
						Msg:  "DAYLIGHT block not in VTIMEZONE block",
						Args: []interface{}{"line", lineCount, "content", line},
					}
				}
				mode = "daylight"
			case "VEVENT":
				if mode == "event" {
					return nil, errNestedBlock("VEVENT", lineCount, line)
				}
				mode = "event"
			case "VALARM":
				if mode == "alarm" {
					return nil, errNestedBlock("VALARM", lineCount, line)
				}
				if mode != "event" {
					return nil, &slogError{
						Msg:  "VALARM block not in VEVENT block",
						Args: []interface{}{"line", lineCount, "content", line},
					}
				}
				if mode == "event" {
					mode = "alarm"
				}
			default:
				if mode == "" {
					return nil, &slogError{
						Msg:  "expecting BEGIN block",
						Args: []interface{}{"line", lineCount, "content", line},
					}
				}
				slog.Debug("unhandled BEGIN block", "line", lineCount, "content", line)
			}
		case "END":
			switch mode {
			case "calendar":
				if value != "VCALENDAR" &&
					value != "VEVENT" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				mode = ""
			case "timezone":
				if value != "VTIMEZONE" &&
					value != "STANDARD" &&
					value != "DAYLIGHT" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				mode = ""
			case "standard":
				if value != "STANDARD" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				mode = ""
			case "daylight":
				if value != "DAYLIGHT" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				mode = ""
			case "event":
				if value != "VEVENT" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				eventCount++
				mode = ""
				if newEvent.summary == "" {
					newEvent.summary = "(no title)"
				}
				if err := cal.AddEvent(newEvent); err != nil {
					return nil, &slogError{
						Msg:  "event validation failed",
						Args: []interface{}{"line", lineCount, "content", line, "err", err},
					}
				}
				newEvent = NewEvent()
			case "alarm":
				if value != "VALARM" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				if err := newEvent.AddAlarm(newAlarm); err != nil {
					return nil, &slogError{
						Msg:  "alarm validation failed",
						Args: []interface{}{"line", lineCount, "content", line, "err", err},
					}
				}
				mode = "event"
			}
		default:
			switch mode {
			case "calendar":
				switch key {
				case "PRODID":
					cal.prodId = value
				case "X-WR-CALNAME":
					cal.SetName(value)
				case "X-WR-CALDESC":
					cal.SetDescription(value)
				default:
					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			case "event":
				switch key {
				case "UID":
					newEvent.id = value
				case "SUMMARY":
					newEvent.SetSummary(value)
				case "DESCRIPTION":
					newEvent.SetDescription(value)
				case "LOCATION":
					newEvent.SetLocation(value)
				case "URL":
					if err := newEvent.SetUrl(value); err != nil {
						return nil, &slogError{
							Msg:  err.Error(),
							Args: []interface{}{"line", lineCount, "content", line},
						}
					}
				case "STATUS":
					switch value {
					case string(EventStatusConfirmed):
						newEvent.SetStatus(EventStatusConfirmed)
					case string(EventStatusCancelled):
						newEvent.SetStatus(EventStatusCancelled)
					case string(EventStatusTentative):
						newEvent.SetStatus(EventStatusCancelled)
					default:
					}
				case "TRANSP":
					switch value {
					case string(EventTransparencyOpaque):
						newEvent.SetTransparency(EventTransparencyOpaque)
					case string(EventTransparencyTransparent):
						newEvent.SetTransparency(EventTransparencyTransparent)
					default:
						return nil, &slogError{
							Msg:  "unhandled transparency",
							Args: []interface{}{"line", lineCount, "content", line},
						}
					}
				case "CREATED":
					parsedDatetime, err := time.Parse("20060102T150405Z", value)
					if err != nil {
						return nil, &slogError{
							Msg:  "can't parse created datetime",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.createdAt = parsedDatetime
				case "LAST-MODIFIED":
					parsedDatetime, err := time.Parse("20060102T150405Z", value)
					if err != nil {
						return nil, &slogError{
							Msg:  "can't parse last-modified datetime",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.updatedAt = parsedDatetime
				case "SEQUENCE":
					parsedInt, err := strconv.Atoi(value)
					if err != nil {
						return nil, &slogError{
							Msg:  "can't parse sequence",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.sequence = parsedInt
				case "RRULE":
					parsedRrule, err := rrule.StrToRRule(value)
					if err != nil {
						return nil, &slogError{
							Msg:  "can't parse rrule",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.SetRRule(parsedRrule)
				case "X-GOOGLE-CONFERENCE":
					if newEvent.url == "" {
						if err := newEvent.SetUrl(value); err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					}
				default:
					switch {
					case strings.HasPrefix(key, "DT"):
						parsedDatetime, err := parseDate(line)
						if err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}

						// set the datetime according to the DT field
						field := strings.Split(key, ";")[0]
						switch strings.TrimPrefix(field, "DT") {
						case "START":
							newEvent.SetStartDate(*parsedDatetime)
						case "END":
							newEvent.SetEndDate(*parsedDatetime)
						case "STAMP":
						default:
							return nil, &slogError{
								Msg:  "unhandled DT field, expecting DTSTART, DTEND, or DTSTAMP",
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						continue
					case strings.HasPrefix(key, "EXDATE"):
						parsedDatetime, err := parseDate(line)
						if err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.AddExDate(*parsedDatetime); err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					case strings.HasPrefix(key, "RDATE"):
						parsedDatetime, err := parseDate(line)
						if err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.AddRDate(*parsedDatetime); err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					case strings.HasPrefix(key, "RECURRENCE-ID"):
						parsedDatetime, err := parseDate(line)
						if err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.SetRecurrenceID(*parsedDatetime); err != nil {
							return nil, &slogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					default:
						slog.Warn("unhandled line", "line", lineCount, "content", line)
					}
				}
			case "alarm": // in VEVENT block
				switch key {
				case "UID":
					if value != "" {
						newAlarm.uid = value
						continue
					}
				case "X-WR-ALARMUID":
					if value != "" && newAlarm.uid == "" {
						newAlarm.uid = value
						continue
					}
				case "ACTION":
					switch value {
					case "AUDIO":
						newAlarm.SetAction(AlarmActionAudio)
					case "DISPLAY":
						newAlarm.SetAction(AlarmActionDisplay)
					case "EMAIL":
						newAlarm.SetAction(AlarmActionEmail)
					case "PROCEDURE":
						newAlarm.SetAction(AlarmActionProcedure)
					default:
						return nil, &slogError{
							Msg:  "unhandled alarm action",
							Args: []interface{}{"line", lineCount, "content", line},
						}
					}
				case "TRIGGER":
					if err := newAlarm.SetTrigger(value); err != nil {
						return nil, &slogError{
							Msg:  "can't set alarm trigger",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
				case "DURATION":
					if err := newAlarm.SetDuration(value); err != nil {
						return nil, &slogError{
							Msg:  "can't set alarm duration",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
				case "REPEAT":
					parsedInt, err := strconv.Atoi(value)
					if err != nil {
						return nil, &slogError{
							Msg:  "can't parse alarm repeat",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newAlarm.SetRepeat(parsedInt)
				case "DESCRIPTION":
					newAlarm.SetDescription(value)
				case "SUMMARY":
					newAlarm.SetSummary(value)
				case "ATTENDEE":
					attendee := Attendee{}
					if err := attendee.Unmarshal(value); err != nil {
						return nil, &slogError{
							Msg:  "can't unmarshal attendee",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newAlarm.AddAttendee(attendee)
				default:
					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			default:
				slog.Warn("unhandled line", "line", lineCount, "content", line)
			}
		}
	}

	slog.Info("parse calendar done", "eventCount", eventCount)

	return &cal, nil
}

func (cal *Calendar) Marshal() (string, *slogError) {
	var sb strings.Builder

	sb.WriteString("BEGIN:VCALENDAR\n")
	sb.WriteString(fmt.Sprintf("PRODID:%s\n", cal.prodId))
	sb.WriteString("VERSION:2.0\n")
	sb.WriteString(fmt.Sprintf("X-WR-CALNAME:%s\n", cal.name))
	if cal.description != "" {
		sb.WriteString(fmt.Sprintf("X-WR-CALDESC:%s\n", cal.description))
	}

	for _, event := range cal.events {
		eventStr, err := event.Marshal()
		if err != nil {
			return "", &utils.SlogError{
				Msg:  "can't marshal event",
				Args: []interface{}{"eventID", event.id, "err", err},
			}
		}
		sb.WriteString(eventStr)
	}
	sb.WriteString("END:VCALENDAR\n")

	return sb.String(), nil
}

func (cal *Calendar) MarshalToFile(path string) *slogError {
	file, err := os.Create(path)
	if err != nil {
		return &slogError{
			Msg:  "can't create file",
			Args: []interface{}{"path", path, "err", err},
		}
	}
	defer file.Close()

	calStr, err2 := cal.Marshal()
	if err2 != nil {
		return err2
	}

	if _, err := file.WriteString(calStr); err != nil {
		return &slogError{
			Msg:  "can't write calendar to file",
			Args: []interface{}{"path", path, "err", err},
		}
	}

	return nil
}

// #region Getters
func (c *Calendar) GetID() string {
	return c.id
}
func (c *Calendar) GetProdID() string {
	return c.prodId
}
func (c *Calendar) GetName() string {
	return c.name
}
func (c *Calendar) GetDescription() string {
	return c.description
}
func (c *Calendar) GetUrl() string {
	return c.url
}
func (c *Calendar) GetEvents() []Event {
	return c.events
}

// #endregion

// #region Setters
func (c *Calendar) SetId(id string) {
	c.id = id
func (c *Calendar) SetName(name string) {
	c.name = name
}
func (c *Calendar) SetDescription(description string) {
	c.description = description
}

// #endregion

// Validate the event and add it to the calendar
func (c *Calendar) AddEvent(event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	c.events = append(c.events, event)
	return nil
}
