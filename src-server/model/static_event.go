package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"towd/src-server/utils"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/xyedo/rrule"
)

// This struct technically is a model, but not for create database table.
//
// StaticEvent represents a fully parsed event (no recurrence rule or
// master/child events) for displaying on Discord or the web client.
type StaticEvent struct {
	ID          string    `json:"id"`
	StartDate   int64     `json:"start_date"`
	EndDate     int64     `json:"end_date"`
	IsWholeDay  bool      `json:"is_whole_day"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	URL         string    `json:"url"`
	Organizer   string    `json:"organizer"`
	Attendees   *[]string `json:"attendees"`
}

// One function call to get statically generated events from a range of dates.
func GetStaticEventInRange(
	ctx context.Context,
	db bun.IDB,
	startDateStartRange int64,
	startDateEndRange int64,
) (*[]StaticEvent, error) {
	staticEvents := make([]StaticEvent, 0)

	masterEvents := make([]MasterEvent, 0)
	if err := db.NewSelect().
		Model(&masterEvents).
		Where("start_date >= ?", startDateStartRange).
		Where("start_date <= ?", startDateEndRange).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("GetStaticEventInRange: %w", err)
	}
	childEvents := make([]ChildEvent, 0)
	if err := db.NewSelect().
		Model(&childEvents).
		Where("start_date >= ?", startDateStartRange).
		Where("start_date <= ?", startDateEndRange).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("GetStaticEventInRange: %w", err)
	}
	// this acts as exdates
	childEventRecIDs := make(map[int64]struct{})
	for _, childEvent := range childEvents {
		childEventRecIDs[childEvent.RecurrenceID] = struct{}{}
	}

	for _, e := range masterEvents {
		attendees := func() []string {
			models := make([]Attendee, 0)
			if err := db.NewSelect().
				Model(&models).
				Where("event_id = ?", e.ID).
				Scan(ctx); err != nil {
				slog.Error("GetStaticEventInRange", "err", err)
			}
			attendees := make([]string, 0)
			for _, attendee := range models {
				attendees = append(attendees, attendee.Data)
			}
			return attendees
		}()

		// add the master event right away and continue if it's not recurring
		if e.RRule == "" {
			staticEvents = append(staticEvents, StaticEvent{
				ID:        e.ID,
				StartDate: e.StartDate,
				EndDate:   e.EndDate,
				IsWholeDay: func() bool {
					startDate := time.Unix(e.StartDate, 0)
					return startDate.Hour() == 0 && startDate.Minute() == 0
				}(),
				Title:       e.Summary,
				Description: e.Description,
				Location:    e.Location,
				URL:         e.URL,
				Organizer:   e.Organizer,
				Attendees:   &attendees,
			})
			continue
		}

		// parse the recurrence rule set
		rruleSet, err := rrule.StrToRRuleSet(e.RRule)
		if err != nil {
			return nil, fmt.Errorf("GetStaticEventInRange: %w", err)
		}

		// rdates AND parsed rrules
		rDates := make(map[int64]struct{})
		if e.RDate != "" {
			for _, dateStr := range strings.Split(e.RDate, ",") {
				dateInt, err := strconv.ParseInt(dateStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("GetStaticEventInRange: %w", err)
				}
				rDates[dateInt] = struct{}{}
			}
		}
		for _, date := range rruleSet.All() {
			rDates[date.Unix()] = struct{}{}
		}

		// exdates of the master event
		exDates := make(map[int64]struct{})
		if e.ExDate != "" {
			for _, dateStr := range strings.Split(e.ExDate, ",") {
				dateInt, err := strconv.ParseInt(dateStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("GetStaticEventInRange: %w", err)
				}
				exDates[dateInt] = struct{}{}
			}
		}

		// iterate and create clones of the master event
		// with different start and end dates
		eventDuration := e.EndDate - e.StartDate
		for date := range rDates {
			if _, ok := exDates[date]; ok {
				continue
			}
			if _, ok := childEventRecIDs[date]; ok {
				continue
			}
			if date < startDateStartRange {
				continue
			}
			staticEvents = append(staticEvents, StaticEvent{
				ID:        e.ID,
				StartDate: date,
				EndDate:   date + eventDuration,
				IsWholeDay: func() bool {
					startDate := time.Unix(date, 0)
					return startDate.Hour() == 0 && startDate.Minute() == 0
				}(),
				Title:       e.Summary,
				Description: e.Description,
				Location:    e.Location,
				URL:         e.URL,
				Organizer:   e.Organizer,
				Attendees:   &attendees,
			})
		}
	}

	for _, e := range childEvents {
		attendees := func() []string {
			models := make([]Attendee, 0)
			if err := db.NewSelect().
				Model(&models).
				Where("event_id = ?", e.RecurrenceID).
				Scan(ctx); err != nil {
				slog.Error("GetStaticEventInRange", "err", err)
			}
			attendees := make([]string, 0)
			for _, attendee := range models {
				attendees = append(attendees, attendee.Data)
			}
			return attendees
		}()
		eventDuration := e.EndDate - e.StartDate
		staticEvents = append(staticEvents, StaticEvent{
			ID:        string(e.RecurrenceID),
			StartDate: e.RecurrenceID,
			EndDate:   e.RecurrenceID + eventDuration,
			IsWholeDay: func() bool {
				startDate := time.Unix(e.RecurrenceID, 0)
				return startDate.Hour() == 0 && startDate.Minute() == 0
			}(),
			Title:       e.Summary,
			Description: e.Description,
			Location:    e.Location,
			URL:         e.URL,
			Organizer:   e.Organizer,
			Attendees:   &attendees,
		})
	}

	// sort the events by start date
	sort.Slice(staticEvents, func(i, j int) bool {
		return staticEvents[i].StartDate < staticEvents[j].StartDate
	})

	return &staticEvents, nil
}

func (s *StaticEvent) FromNaturalText(ctx context.Context, as *utils.AppState, text string) error {
	apiKey := as.Config.GetGroqApiKey()
	if apiKey == "" {
		return fmt.Errorf("FromNaturalText: api key is blank")
	}

	// #region | preparing the request body
	now := time.Now().UTC().Truncate(24 * time.Hour).Format("02/01/2006 15:04")
	slog.Debug("FromNaturalText", "now", now, "text", text)
	reqBody := struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Model          string  `json:"model"`
		Temperature    float64 `json:"temperature"`
		MaxTokens      int     `json:"max_tokens"`
		TopP           float64 `json:"top_p"`
		Stream         bool    `json:"stream"`
		ResponseFormat struct {
			Type string `json:"type"`
		} `json:"response_format"`
	}{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{
				Role:    "system",
				Content: "create json represent calendar event from user input, contain these fields: success, title, description, start, end, location, url, attendee; attendee always a list, success is boolean, everything else are strings; current time for parse relative datetime provided at begin of input in format DD/MM/YYYY hh:mm, start/end date use the same format (strictly follow the format, do not add seconds); if no end date, assume event ends in 1 hour; for whole-day event, set hh:mm to 00:00; title and start date are required, set success to false if any of them is missing and put the reason in description",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("It's %s. %s", now, text),
			},
		},
		Model:       "llama3-8b-8192",
		Temperature: 1,
		MaxTokens:   1024,
		TopP:        1,
		Stream:      false,
		ResponseFormat: struct {
			Type string `json:"type"`
		}{
			Type: "json_object",
		},
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("FromNaturalText: marshal req body: %w", err)
	}
	// #endregion

	// #region | send the request
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return fmt.Errorf("FromNaturalText: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("FromNaturalText: do request: %w", err)
	}
	// #endregion

	// #region | read the response
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FromNaturalText: bad status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("FromNaturalText: read body: %w", err)
	}
	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &respBody); err != nil {
		return fmt.Errorf("FromNaturalText: unmarshal body: %w", err)
	}
	// #endregion

	// #region | assign the response content
	if len(respBody.Choices) == 0 {
		return fmt.Errorf("FromNaturalText: no choices")
	}
	if len(respBody.Choices[0].Message.Content) == 0 {
		return fmt.Errorf("FromNaturalText: no content")
	}
	var respContent struct {
		Success     bool     `json:"success"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Start       string   `json:"start"`
		End         string   `json:"end"`
		Location    string   `json:"location"`
		URL         string   `json:"url"`
		Attendees   []string `json:"attendees"`
	}
	if err := json.Unmarshal([]byte(respBody.Choices[0].Message.Content), &respContent); err != nil {
		return fmt.Errorf("FromNaturalText: unmarshal content: %w", err)
	}

	if !respContent.Success {
		return fmt.Errorf("FromNaturalText: %s", respContent.Description)
	}

	// basic info
	s.ID = uuid.NewString()
	s.Title = utils.CleanupString(respContent.Title)
	s.Description = utils.CleanupString(respContent.Description)

	// datetime related
	slog.Debug("FromNaturalText", "start", respContent.Start, "end", respContent.End)
	startDate, err := time.ParseInLocation("02/01/2006 15:04", respContent.Start, as.Config.GetLocation())
	if err != nil {
		return fmt.Errorf("FromNaturalText: parse start date: %w", err)
	}
	s.StartDate = startDate.Unix()
	endDate, err := time.ParseInLocation("02/01/2006 15:04", respContent.End, as.Config.GetLocation())
	if err != nil {
		return fmt.Errorf("FromNaturalText: parse end date: %w", err)
	}
	s.EndDate = endDate.Unix()

	// additional info
	s.Location = utils.CleanupString(respContent.Location)
	s.URL = respContent.URL
	s.Attendees = &respContent.Attendees
	// #endregion

	return nil
}
