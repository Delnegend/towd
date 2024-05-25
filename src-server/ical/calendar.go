package ical

import (
	"bufio"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"towd/src-server/utils"

	"github.com/google/uuid"
	"github.com/xyedo/rrule"
)

type Calendar struct {
	id          string
	name        string
	description string
	version     string
	url         string
	events      []Event
}

func NewCalendar() Calendar {
	return Calendar{
		id: uuid.NewString(),
	}
}

func FromFile(path string) (*Calendar, *utils.SlogError) {
	cal := NewCalendar()

	file, err := os.Open(path)
	if err != nil {
		return nil, &utils.SlogError{
			Msg: "error opening file",
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var mode, lastLine string
	var lineCount, eventCount int

	newEvent := NewEvent()

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		if strings.HasPrefix(line, " ") {
			line = lastLine + strings.TrimPrefix(line, " ")
			lastLine = line
			continue
		} else {
			lastLine = line
		}

		if strings.HasPrefix(line, "ATTACH") ||
			strings.HasPrefix(line, "ATTENDEE") ||
			strings.HasPrefix(line, "X-GOOGLE-CONFERENCE") {
			continue
		}

		slice := strings.Split(line, ":")
		if len(slice) < 2 {
			return nil, &utils.SlogError{
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
					return nil, &utils.SlogError{
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
					return nil, &utils.SlogError{
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
					return nil, &utils.SlogError{
						Msg:  "VALARM block not in VEVENT block",
						Args: []interface{}{"line", lineCount, "content", line},
					}
				}
				if mode == "event" {
					mode = "alarm"
				}
			default:
				if mode == "" {
					return nil, &utils.SlogError{
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

				if err := newEvent.Validate(); err != nil {
					return nil, &utils.SlogError{
						Msg:  "event validation failed",
						Args: []interface{}{"line", lineCount, "content", line, "err", err},
					}
				}
				cal.AddEvent(newEvent)
				newEvent = NewEvent()
			case "alarm":
				if value != "VALARM" {
					return nil, errUnexpectedEnd(lineCount, line)
				}
				mode = "event"
			}
		default:
			switch mode {
			case "calendar":
				switch key {
				case "PRODID":
					cal.id = value
				case "VERSION":
					cal.SetVersion(value)
				case "X-WR-CALNAME":
					cal.SetName(value)
				case "X-WR-CALDESC":
					cal.SetDescription(value)
				case "X-WR-TIMEZONE":
					cal.SetTimezone(value)
				default:
					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			case "timezone":
				switch key {
				case "TZID":
				default:
					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			case "standard": // in timezone block
			case "daylight": // in timezone block
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
						return nil, &utils.SlogError{
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
						return nil, &utils.SlogError{
							Msg:  "unhandled transparency",
							Args: []interface{}{"line", lineCount, "content", line},
						}
					}
				case "ATTENDEE": // TODO: properly handle above
				case "CREATED":
					parsedDatetime, err := time.Parse("20060102T150405Z", value)
					if err != nil {
						return nil, &utils.SlogError{
							Msg:  "can't parse created datetime",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.createdAt = parsedDatetime
				case "LAST-MODIFIED":
					parsedDatetime, err := time.Parse("20060102T150405Z", value)
					if err != nil {
						return nil, &utils.SlogError{
							Msg:  "can't parse last-modified datetime",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.updatedAt = parsedDatetime
				case "ATTACH": // TODO: properly handle above
				case "SEQUENCE":
					parsedInt, err := strconv.Atoi(value)
					if err != nil {
						return nil, &utils.SlogError{
							Msg:  "can't parse sequence",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.sequence = parsedInt
				case "RRULE":
					parsedRrule, err := rrule.StrToRRule(value)
					if err != nil {
						return nil, &utils.SlogError{
							Msg:  "can't parse rrule",
							Args: []interface{}{"line", lineCount, "content", line, "err", err},
						}
					}
					newEvent.SetRRule(parsedRrule)
				default:
					switch {
					case strings.HasPrefix(key, "DT"):
						parsedDatetime, err := parseDate(value)
						if err != nil {
							return nil, &utils.SlogError{
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
							return nil, &utils.SlogError{
								Msg:  "unhandled DT field, expecting DTSTART, DTEND, or DTSTAMP",
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						continue
					case strings.HasPrefix(key, "EXDATE"):
						parsedDatetime, err := parseDate(value)
						if err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.AddExDate(*parsedDatetime); err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					case strings.HasPrefix(key, "RDATE"):
						parsedDatetime, err := parseDate(value)
						if err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.AddRDate(*parsedDatetime); err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					case strings.HasPrefix(key, "RECURRENCE-ID"):
						parsedDatetime, err := parseDate(value)
						if err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
						if err := newEvent.SetRecurrenceID(*parsedDatetime); err != nil {
							return nil, &utils.SlogError{
								Msg:  err.Error(),
								Args: []interface{}{"line", lineCount, "content", line},
							}
						}
					}

					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			case "alarm": // in alarm block
			default:
				slog.Warn("unhandled line", "line", lineCount, "content", line)
			}
		}
	}

	slog.Info("parse calendar done", "eventCount", eventCount)

	return &cal, nil
}

// #region Getters
func (c *Calendar) GetId() string {
	return c.id
}
func (c *Calendar) GetName() string {
	return c.name
}
func (c *Calendar) GetDescription() string {
	return c.description
}
func (c *Calendar) GetVersion() string {
	return c.version
}
func (c *Calendar) GetUrl() string {
	return c.url
}

// #endregion

// #region Setters
func (c *Calendar) SetName(name string) {
	c.name = name
}
func (c *Calendar) SetDescription(description string) {
	c.description = description
}
func (c *Calendar) SetVersion(version string) {
	c.version = version
}

func (c *Calendar) SetUrl(url string) {
	c.url = url
}

// #endregion

func (c *Calendar) AddEvent(event Event) {
	c.events = append(c.events, event)
}
