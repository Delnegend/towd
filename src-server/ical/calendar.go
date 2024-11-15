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
// Parse from a URL
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
	"regexp"
	"strings"
	"towd/src-server/ical/event"
	"towd/src-server/ical/structured"

	"github.com/google/uuid"
)

// The main struct of the package
type Calendar struct {
	id           string
	prodID       string
	name         string
	description  string
	masterEvents map[string]*event.MasterEvent

	// this field only serve ONE PURPOSE: temporary storage for child events
	// that are not yet added to a master event. This is to prevent adding
	// child events to a master event that is not yet parsed.
	childEvents []*event.ChildEvent
}

// Initialize a new Calendar{} struct
func NewCalendar() Calendar {
	return Calendar{
		id:           uuid.NewString(),
		masterEvents: make(map[string]*event.MasterEvent),
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
		defer close(lineCh)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if lineCh != nil {
				lineCh <- scanner.Text()
			}
		}
	}()

	return iCalParser(lineCh)
}

// Unmarshal an iCalendar URL into a Calendar{} struct.
func FromIcalUrl(url_ string) (*Calendar, *CustomError) {
	if _, err := url.ParseRequestURI(url_); err != nil {
		return nil, NewCustomError("can't parse URL", map[string]any{
			"url": url_,
			"err": err,
		})
	}

	req, err := http.NewRequest("GET", url_, nil)
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
		defer close(lineCh)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	return iCalParser(lineCh)
}

// The shared logic for parsing iCalendar files from a channel of strings, which
// is used by FromIcalFile and FromIcalUrl.
func iCalParser(lineCh <-chan string) (*Calendar, *CustomError) {
	ignoredFields := map[string]struct{}{
		"X-APPLE-TRAVEL-ADVISORY-BEHAVIOR": {},
		"ACKNOWLEDGED":                     {},
		"X-APPLE-DEFAULT-ALARM":            {},
		"VERSION":                          {},
		"CALSCALE":                         {},
		"METHOD":                           {},
		"X-WR-TIMEZONE":                    {},
	}

	cal := NewCalendar()
	lineCount := 0
	eventCount := 0

	errCh := make(chan *CustomError)

	go func() {
		var mode string
		undecidedEvent := event.NewUndecidedEvent()
		newAlarm := structured.NewAlarm()

		var line string
		isFirstLine := true
	scoped:
		for rawLine := range lineCh {
			lineCount++
			switch {
			case isFirstLine:
				isFirstLine = false
				line = rawLine
				continue
			case strings.HasPrefix(rawLine, " "):
				line += rawLine
				continue
			case rawLine == "END:VCALENDAR":
				errCh <- nil
				break scoped
			}

			slice := strings.SplitN(line, ":", 2)
			if len(slice) < 2 {
				switch mode {
				case "event":
					if err := undecidedEvent.AddIcalProperty(line); err != nil {
						errCh <- NewCustomError("can't add ical property to event", map[string]any{
							"line":    lineCount,
							"content": line,
							"err":     err,
						})
					}
				case "alarm":
					newAlarm.AddIcalProperty(line)
				default:
					errCh <- NewCustomError("unhandled line", map[string]any{
						"line":    lineCount,
						"content": line,
					})
				}
				line = rawLine
				continue
			}
			key := strings.ToUpper(strings.TrimSpace(slice[0]))
			value := strings.TrimSpace(slice[1])

			if _, ok := ignoredFields[key]; ok {
				line = rawLine
				continue
			}

			switch key {
			case "BEGIN":
				switch value {
				case "VCALENDAR":
					if mode == "calendar" {
						errCh <- NewCustomError("nested VCALENDAR block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					mode = "calendar"
				case "VTIMEZONE":
					if mode == "timezone" {
						errCh <- NewCustomError("nested VTIMEZONE block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					mode = "timezone"
				case "STANDARD":
					if mode == "standard" {
						errCh <- NewCustomError("nested STANDARD block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					if mode != "timezone" {
						errCh <- NewCustomError("STANDARD block not in VTIMEZONE block", map[string]any{
							"line":    lineCount,
							"content": line,
							"mode":    mode,
						})
					}
					mode = "standard"
				case "DAYLIGHT":
					switch {
					case mode == "timezone":
						mode = "daylight"
					case mode == "daylight":
						errCh <- NewCustomError("nested DAYLIGHT block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					default:
						errCh <- NewCustomError("DAYLIGHT block not in STANDARD block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
				case "VEVENT":
					if mode == "event" {
						slog.Warn("nested VEVENT block", "line", lineCount, "content", line)
					}
					mode = "event"
				case "VALARM":
					switch {
					case mode == "event":
						mode = "alarm"
					case mode == "alarm":
						errCh <- NewCustomError("nested VALARM block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					default:
						errCh <- NewCustomError("VALARM block not in VEVENT block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
				default:
					if mode == "" {
						errCh <- NewCustomError("expecting BEGIN block", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
				}
			case "END":
				switch mode {
				case "timezone":
					if value != "VTIMEZONE" &&
						value != "STANDARD" &&
						value != "DAYLIGHT" {
						errCh <- NewCustomError("unexpected END:VTIMEZONE", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					mode = "calendar"
				case "standard":
					if value != "STANDARD" {
						errCh <- NewCustomError("unexpected END:STANDARD", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					mode = "timezone"
				case "daylight":
					if value != "DAYLIGHT" {
						errCh <- NewCustomError("unexpected END:DAYLIGHT", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					mode = "timezone"
				case "event":
					mode = "calendar"
					if value != "VEVENT" {
						errCh <- NewCustomError("unexpected END:VEVENT", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					eventCount++
					if undecidedEvent.GetSummary() == "" {
						undecidedEvent.SetSummary("(no title)")
					}
					resultEvent, err := undecidedEvent.DecideEventType()
					if err != nil {
						errCh <- NewCustomError("can't decide event type", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					switch decidedEvent := resultEvent.(type) {
					case event.MasterEvent:
						if _, ok := cal.masterEvents[undecidedEvent.GetID()]; ok {
							errCh <- NewCustomError("duplicate event id", map[string]any{
								"line":    lineCount,
								"content": line,
							})
						}
						cal.masterEvents[undecidedEvent.GetID()] = &decidedEvent
					case event.ChildEvent:
						cal.childEvents = append(cal.childEvents, &decidedEvent)
					default:
						errCh <- NewCustomError("can't decide event type", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					undecidedEvent = event.NewUndecidedEvent()
				case "alarm":
					if value != "VALARM" {
						errCh <- NewCustomError("unexpected END:VALARM", map[string]any{
							"line":    lineCount,
							"content": line,
						})
					}
					undecidedEvent.AddAlarm(newAlarm)
					newAlarm = structured.NewAlarm()
					mode = "event"
				default:
					errCh <- NewCustomError("unexpected END", map[string]any{
						"line":    lineCount,
						"content": line,
						"mode":    mode,
					})
				}
			default:
				switch mode {
				case "timezone", "standard", "daylight":
				case "calendar":
					switch key {
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
					if err := undecidedEvent.AddIcalProperty(line); err != nil {
						errCh <- NewCustomError("can't add ical property to event", map[string]any{
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
			line = rawLine
		}
	}()

	err := <-errCh
	if err != nil {
		return nil, err
	}

	// add child events to master events
	for _, childEvent := range cal.childEvents {
		if masterEvent, ok := cal.masterEvents[childEvent.GetID()]; ok {
			if err := masterEvent.AddChildEvent(childEvent); err != nil {
				slog.Warn("can't add child event to master event", "childEventID", childEvent.GetID(), "err", err)
				continue
			}
		} else {
			slog.Warn("master event not found for child event", "masterEventID", childEvent.GetID())
			continue
		}
	}
	cal.childEvents = nil

	return &cal, nil
}

// Marshal a Calendar{} struct into an iCalendar string.
func (cal *Calendar) ToIcal(w func(string)) error {
	writer := func(s string) {
		var slices []string
		for i := 0; i < len(s); i += 75 {
			end := min(i+75, len(s))
			slices = append(slices, s[i:end])
		}
		w(strings.Join(slices, "\n "))
	}

	writer("BEGIN:VCALENDAR\n")
	writer("PRODID:" + cal.prodID + "\n")
	writer("VERSION:2.0\n")
	writer("X-WR-CALNAME:" + cal.name + "\n")
	if cal.description != "" {
		writer("X-WR-CALDESC:" + cal.description + "\n")
	}

	for _, event := range cal.masterEvents {
		event.ToIcal(writer)
	}
	writer("END:VCALENDAR\n")

	return nil
}

// Get the calendar ID
func (c *Calendar) GetID() string {
	return c.id
}

// Set the calendar ID
func (c *Calendar) SetID(id string) {
	c.id = id
}

// Get the calendar ProdID
func (c *Calendar) GetProdID() string {
	return c.prodID
}

// Set the calendar ProdID
func (c *Calendar) SetProdID(prodID string) error {
	rgx := regexp.MustCompile(`^-//\w+//\w+//\w+$`)
	if !rgx.MatchString(prodID) {
		return fmt.Errorf("prodID must match RFC5545 format (-//ORG/ORGUNIT/APPNAME)")
	}

	c.prodID = prodID
	return nil
}

// Get the calendar name
func (c *Calendar) GetName() string {
	return c.name
}

// Set the calendar name
func (c *Calendar) SetName(name string) {
	c.name = name
}

// Get the calendar description
func (c *Calendar) GetDescription() string {
	return c.description
}

// Set the calendar description
func (c *Calendar) SetDescription(desc string) {
	c.description = desc
}

// Add a MasterEvent to the calendar
func (c *Calendar) AddMasterEvent(id string, e *event.MasterEvent) error {
	if _, ok := c.masterEvents[id]; ok {
		return fmt.Errorf("event with id %s already exists", id)
	}
	c.masterEvents[id] = e
	return nil
}

func (c *Calendar) RemoveMasterEvent(id string) error {
	if _, ok := c.masterEvents[id]; !ok {
		return fmt.Errorf("event with id %s does not exist", id)
	}
	delete(c.masterEvents, id)
	return nil
}

// Iterate over all MasterEvents in the calendar and apply a function to each.
func (c *Calendar) IterateMasterEvents(f func(id string, event *event.MasterEvent) error) error {
	for id, event := range c.masterEvents {
		if err := f(id, event); err != nil {
			return err
		}
	}
	return nil
}

// Get the number of MasterEvents in the calendar
func (c *Calendar) GetMasterEventCount() int {
	return len(c.masterEvents)
}
