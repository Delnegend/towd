// The `ical` package parse and serialize iCalendar files.
//
// # References:
// - RFC5545: https://datatracker.ietf.org/doc/html/rfc5545
// - RFC6321: https://datatracker.ietf.org/doc/html/rfc6321
//
// # Notes:
// - Not all properties are supported when parsing, instead stored in the custom
//   property array for serialization back into iCalendar format if needed.
// - VTIMEZONE and VALARM sections, including their sub-sections, are ignored.
//   parsing local timezones are still supported. All datetimes are stored in UTC.
//
// - There are 3 types of events: MasterEvent, ChildEvent and UndecidedEvent.
//   - MasterEvent: a "normal" event.
//   - ChildEvent: modify a recurring MasterEvent.
//   - UndecidedEvent: a placeholder for a future Master/ChildEvent.
// - Calendar{} only holds MasterEvent and ChildEvent, read-only and guaranteed
//   to be valid.
//
// # Example usage:
//
// ## Working with a Calendar struct
//
// Parse from a file
//	calendar, _ := ical.FromIcalFile("path/to/input/calendar.ics")
//
// Parse from an URL
//	calendar, _ := ical.FromIcalUrl("https://example.com/calendar.ics")
//
// Marshal to a string -> file
//	output, _ := calendar.ToIcal()
//	_ := os.WriteFile("path/to/output/calendar.ics", []byte(output), 0644)
//
// Create a new Calendar struct
//	calendar := ical.NewCalendar()
//
// ## Working with MasterEvent, ChildEvent and UndecidedEvent
//
// Create a new UndecidedEvent
//	undecidedEvent := ical.NewUndecidedEvent()
//
// Turn into a Child/MasterEvent
//	event, _ := undecidedEvent.DecideEventType()
//
// Add a ChildEvent to a MasterEvent
//	masterEvent.AddChildEvent(event)
//
// Add a MasterEvent to a Calendar
//	calendar.AddMasterEvent(event)

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

// The main struct of the package
type Calendar struct {
	id           string
	prodID       string
	name         string
	description  string
	masterEvents map[string]event.MasterEvent
	childEvents  map[string]event.ChildEvent
}

// Initialize a new Calendar{} struct
func NewCalendar() Calendar {
	return Calendar{
		id:           uuid.NewString(),
		masterEvents: make(map[string]event.MasterEvent),
		childEvents:  make(map[string]event.ChildEvent),
	}
}

// Unmarshal an iCalendar file into a Calendar{} struct.
func FromIcalFile(path string) (*Calendar, *CustomError) {
	file, err := os.Open(path)
	if err != nil {
		return nil, NewCustomError("can't opening file", map[string]any{
			"path": path,
			"err":  err,
		})
	}
	defer file.Close()

	lineCh := make(chan string)

	go func() {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	return iCalParser(lineCh)
}

// Unmarshal an iCalendar URL into a Calendar{} struct.
func FromIcalUrl(url_ string) (*Calendar, *CustomError) {
	validUrl, err := url.ParseRequestURI(url_)
	if err != nil {
		return nil, NewCustomError("can't parse URL", map[string]any{
			"url": url_,
			"err": err,
		})
	}

	req, err := http.NewRequest("GET", validUrl.String(), nil)
	if err != nil {
		return nil, NewCustomError("can't create HTTP request", map[string]any{
			"url": url_,
			"err": err,
		})
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, NewCustomError("can't make HTTP request", map[string]any{
			"url": url_,
			"err": err,
		})
	}
	defer resp.Body.Close()

	lineCh := make(chan string)

	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	return iCalParser(lineCh)
}

// The shared logic for parsing iCalendar files from a channel of strings, which
// is used by FromIcalFile and FromIcalUrl.
func iCalParser(lineCh chan string) (*Calendar, *CustomError) {
	defer close(lineCh)

	ignoredFields := map[string]struct{}{
		"X-APPLE-TRAVEL-ADVISORY-BEHAVIOR": {},
		"ACKNOWLEDGED":                     {},
		"X-APPLE-DEFAULT-ALARM":            {},
	}

	cal := NewCalendar()
	var mode string
	lineCount := 0
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

	blankEvent := event.NewUndecidedEvent()
	newAlarm := structured.NewAlarm()

	for line := range mergedLineCh {
		slice := strings.SplitN(line, ":", 2)
		if len(slice) != 2 {
			switch mode {
			case "event":
				if err := blankEvent.AddIcalProperty(line); err != nil {
					return nil, NewCustomError("can't add ical property to event", map[string]any{
						"line":    lineCount,
						"content": line,
						"err":     err,
					})
				}
			case "alarm":
				newAlarm.AddIcalProperty(line)
			default:
				return nil, NewCustomError("unhandled line", map[string]any{
					"line":    lineCount,
					"content": line,
				})
			}
		}
		key := strings.ToUpper(strings.TrimSpace(slice[0]))
		value := strings.TrimSpace(slice[1])

		if _, ok := ignoredFields[key]; ok {
			continue
		}

		switch key {
		case "BEGIN":
			switch value {
			case "VCALENDAR":
				if mode == "calendar" {
					return nil, NewCustomError("nested VCALENDAR block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = "calendar"
			case "VTIMEZONE":
				if mode == "timezone" {
					return nil, NewCustomError("nested VTIMEZONE block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = "timezone"
			case "STANDARD":
				if mode == "standard" {
					return nil, NewCustomError("nested STANDARD block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				if mode != "timezone" {
					return nil, NewCustomError("STANDARD block not in VTIMEZONE block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = "standard"
			case "DAYLIGHT":
				switch {
				case mode == "standard":
					mode = "daylight"
				case mode == "daylight":
					return nil, NewCustomError("nested DAYLIGHT block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				default:
					return nil, NewCustomError("DAYLIGHT block not in STANDARD block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
			case "VEVENT":
				if mode == "event" {
					return nil, NewCustomError("nested VEVENT block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = "event"
			case "VALARM":
				switch {
				case mode == "alarm":
					return nil, NewCustomError("nested VALARM block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				case mode == "event":
					mode = "alarm"
				default:
					return nil, NewCustomError("VALARM block not in VEVENT block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
			default:
				if mode == "" {
					return nil, NewCustomError("expecting BEGIN block", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				slog.Debug("unhandled BEGIN block", "line", lineCount, "content", line)
			}
		case "END":
			switch mode {
			case "calendar":
				if value != "VCALENDAR" &&
					value != "VEVENT" {
					// return nil, errUnexpectedEnd(lineCount, line)
					return nil, NewCustomError("unexpected END:VCALENDAR", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = ""
			case "timezone":
				if value != "VTIMEZONE" &&
					value != "STANDARD" &&
					value != "DAYLIGHT" {
					return nil, NewCustomError("unexpected END:VTIMEZONE", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = ""
			case "standard":
				if value != "STANDARD" {
					return nil, NewCustomError("unexpected END:STANDARD", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = ""
			case "daylight":
				if value != "DAYLIGHT" {
					return nil, NewCustomError("unexpected END:DAYLIGHT", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				mode = ""
			case "event":
				if value != "VEVENT" {
					return nil, NewCustomError("unexpected END:VEVENT", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				eventCount++
				mode = ""
				if blankEvent.GetSummary() == "" {
					blankEvent.SetSummary("(no title)")
				}
				resultEvent, err := blankEvent.DecideEventType()
				if err != nil {
					return nil, NewCustomError("can't decide event type", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				switch resultEvent := resultEvent.(type) {
				case event.MasterEvent:
					if _, ok := cal.masterEvents[blankEvent.GetID()]; ok {
						return nil, NewCustomError("duplicate event id", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					cal.masterEvents[blankEvent.GetID()] = resultEvent
				case event.ChildEvent:
					cal.childEvents[blankEvent.GetID()] = resultEvent
				default:
					return nil, NewCustomError("can't decide event type", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				blankEvent = event.NewUndecidedEvent()

			case "alarm":
				if value != "VALARM" {
					return nil, NewCustomError("unexpected END:VALARM", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				blankEvent.AddAlarm(newAlarm)
				mode = "event"
				newAlarm = structured.NewAlarm()
			}
		default:
			switch mode {
			case "timezone", "standard", "daylight":
			case "calendar":
				switch key {
				case "VERSION", "CALSCALE", "METHOD", "X-WR-TIMEZONE":
				case "PRODID":
					cal.prodID = value
				case "X-WR-CALNAME":
					cal.SetName(value)
				case "X-WR-CALDESC":
					cal.SetDescription(value)
				default:
					slog.Warn("unhandled line", "line", lineCount, "content", line)
				}
			case "event":
				if err := blankEvent.AddIcalProperty(line); err != nil {
					return nil, NewCustomError("can't add ical property to event", map[string]any{
						"line":    lineCount,
						"content": line,
						"err":     err,
					})
				}
			case "alarm":
				newAlarm.AddIcalProperty(line)
			default:
				slog.Warn("unhandled line", "line", lineCount, "content", line)
			}
		}
	}

	validChildEvents := make(map[string]event.ChildEvent)
	for _, childEvent := range cal.childEvents {
		if _, ok := cal.masterEvents[childEvent.GetID()]; ok {
			validChildEvents[childEvent.GetID()] = childEvent
			continue
		}
	}
	cal.childEvents = validChildEvents
	for _, childEvent := range cal.childEvents {
		if masterEvent, ok := cal.masterEvents[childEvent.GetID()]; ok {
			if err := masterEvent.AddChildEvent(&childEvent); err != nil {
				slog.Warn("can't add child event to master event", "childEventID", childEvent.GetID(), "err", err)
				continue
			}
			continue
		}
	}

	return &cal, nil
}

// Marshal a Calendar{} struct into an iCalendar string.
func (cal *Calendar) ToIcal() (string, *CustomError) {
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
			return "", NewCustomError("can't marshal event", map[string]any{
				"eventID": event.GetID(),
				"err":     err,
			})
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
// Get the calendar ID

	calStr, err2 := cal.Marshal()
	if err2 != nil {
		return err2
	}
// Set the calendar ID

	if _, err := file.WriteString(calStr); err != nil {
		return &slogError{
			Msg:  "can't write calendar to file",
			Args: []interface{}{"path", path, "err", err},
		}
// Get the calendar ProdID
// Set the calendar ProdID
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
// Get the calendar name
func (c *Calendar) GetName() string {
	return c.name
}
// Set the calendar name
// Get the calendar description
func (c *Calendar) GetDescription() string {
	return c.description
}
func (c *Calendar) GetUrl() string {
	return c.url
// Set the calendar description
}
func (c *Calendar) GetEvents() []Event {
	return c.events
// Add a MasterEvent to the calendar
}

// #endregion

// #region Setters
func (c *Calendar) SetId(id string) {
	c.id = id
// Iterate over all MasterEvents in the calendar and apply a function to each.
}
func (c *Calendar) SetName(name string) {
	c.name = name
// Iterate over all ChildEvents in the calendar and apply a function to each.
}
func (c *Calendar) SetDescription(description string) {
	c.description = description
// Get the number of MasterEvents in the calendar
}

// #endregion
// Get the number of ChildEvents in the calendar

// Validate the event and add it to the calendar
func (c *Calendar) AddEvent(event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	c.events = append(c.events, event)
	return nil
// Get a MasterEvent from the calendar by ID
// Get a ChildEvent from the calendar by ID
}
