package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"towd/src-server/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type EventIDCtxKeyType string

const EventIDCtxKey EventIDCtxKeyType = "event-id"

type Event struct {
	bun.BaseModel `bun:"table:events"`

	ID          string `bun:"id,pk"`           // required
	Summary     string `bun:"summary,notnull"` // required
	Description string `bun:"description"`
	Location    string `bun:"location"`
	URL         string `bun:"url"`
	Organizer   string `bun:"organizer"`

	StartDateUnixUTC int64 `bun:"start_date,notnull"` // required
	EndDateUnixUTC   int64 `bun:"end_date,notnull"`   // required
	IsWholeDay       bool  `bun:"is_whole_day"`

	CreatedAt int64 `bun:"created_at,notnull"`
	UpdatedAt int64 `bun:"updated_at"`
	Sequence  int   `bun:"sequence"`

	CalendarID string `bun:"calendar_id,notnull"` // required
	ChannelID  string `bun:"channel_id,notnull"`  // required

	Attendees        []*Attendee       `bun:"rel:has-many,join:id=event_id"`
	Calendar         *Calendar         `bun:"rel:belongs-to,join:calendar_id=channel_id"`
	ExternalCalendar *ExternalCalendar `bun:"rel:belongs-to,join:calendar_id=id"`
}

var _ bun.AfterDeleteHook = (*Event)(nil)

func (e *Event) AfterDelete(ctx context.Context, query *bun.DeleteQuery) error {
	if query.DB() == nil {
		return fmt.Errorf("(*Event).AfterDelete: db is nil")
	}

	// getting just-deleted-event-ids from context
	eventIDs := make([]string, 0)
	switch eventID := ctx.Value(EventIDCtxKey).(type) {
	case string:
		if eventID == "" {
			return fmt.Errorf("(*Event).AfterDelete: deletedEventID is blank")
		}
		eventIDs = append(eventIDs, eventID)
	case []string:
		if len(eventID) == 0 {
			return nil
		}
		eventIDs = append(eventIDs, eventID...)
	case nil:
		return fmt.Errorf("(*Event).AfterDelete: event id is nil")
	default:
		return fmt.Errorf("(*Event).AfterDelete: wrong eventID type | type=%T", eventID)
	}

	// delete all related Attendee models
	if _, err := query.DB().NewDelete().
		Model((*Attendee)(nil)).
		Where("event_id IN (?)", bun.In(eventIDs)).
		Exec(ctx); err != nil {
		return fmt.Errorf("(*Event).AfterDelete: can't delete attendees: %w", err)
	}

	return nil
}

func (e *Event) Upsert(ctx context.Context, db bun.IDB) error {
	switch {
	case e.ID == "":
		return fmt.Errorf("(*Event).Upsert: event id is blank")
	case e.Summary == "":
		return fmt.Errorf("(*Event).Upsert: summary is blank")
	case e.StartDateUnixUTC == 0:
		return fmt.Errorf("(*Event).Upsert: start date is blank")
	case e.EndDateUnixUTC == 0:
		return fmt.Errorf("(*Event).Upsert: end date is blank")
	case e.StartDateUnixUTC > e.EndDateUnixUTC:
		return fmt.Errorf("(*Event).Upsert: start date must be before end date")
	case e.URL != "":
		if _, err := url.ParseRequestURI(e.URL); err != nil {
			return fmt.Errorf("(*Event).Upsert: url is invalid: %w", err)
		}
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().UTC().Unix()
	}
	startDateDate := time.Unix(e.StartDateUnixUTC, 0)
	if startDateDate.Hour() == 0 &&
		startDateDate.Minute() == 0 &&
		startDateDate.Second() == 0 {
		e.IsWholeDay = true
	}

	exists, err := db.NewSelect().
		Model((*Event)(nil)).
		Where("id = ?", e.ID).
		Exists(context.Background())
	if err != nil {
		return fmt.Errorf("(*Event).Upsert: %w", err)
	}

	switch exists {
	case true:
		e.UpdatedAt = time.Now().UTC().Unix()
		e.Sequence++
		if _, err := db.NewUpdate().
			Model(e).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*Event).Upsert: %w", err)
		}
	case false:
		if _, err := db.NewInsert().
			Model(e).
			Exec(ctx); err != nil {
			return fmt.Errorf("(*Event).Upsert: %w", err)
		}
	}

	return nil
}

func (e *Event) ToDiscordEmbed(ctx context.Context, db bun.IDB) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       e.Summary,
		Description: e.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Start Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.StartDateUnixUTC),
				Inline: true,
			},
			{
				Name:   "End Date",
				Value:  fmt.Sprintf("<t:%d:f>", e.EndDateUnixUTC),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: e.ID,
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name: e.Organizer,
		},
	}
	if e.Location != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Location",
			Value: e.Location,
		})
	}
	if e.URL != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "URL",
			Value: e.URL,
		})
	}

	if len(e.Attendees) > 0 {
		attendeeStr := make([]string, len(e.Attendees))
		for i, attendee := range e.Attendees {
			attendeeStr[i] = attendee.Data
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Invitees",
			Value: strings.Join(attendeeStr, ", "),
		})
	}

	return embed
}

func (e *Event) FromNaturalText(ctx context.Context, as *utils.AppState, text string) ([]Attendee, error) {
	apiKey := as.Config.GetGroqApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("FromNaturalText: api key is blank")
	}

	// #region | preparing the request body
	now := time.Now().UTC().Truncate(24 * time.Hour).Format("02/01/2006 15:04")
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
				Role: "system",
				Content: `<task>Create a JSON representation of a calendar event from user input.</task>
<field>
- success: boolean
- title: string (required)
- description: string
- start: string (required, format: DD/MM/YYYY hh:mm)
- end: string (format: DD/MM/YYYY hh:mm)
- location: string
- url: string
- attendee: list of strings (anyone mentioned in the content must be included)
</field>
<rules>
- Current time: Provided at the beginning of the input in the format DD/MM/YYYY hh:mm.
- Date format: Use the format DD/MM/YYYY hh:mm strictly (do not add seconds).
- End date: If no end date is provided, assume the event ends in 1 hour.
- Whole-day event: Set hh:mm to 00:00.
- Required fields: Title and start date are required. Set success to false if any of them is missing and put the reason in the description.
</rules>
<example>
<current-time>15/09/2024 14:00</current-time>
<input>Meeting with John on 15/09/2024 14:00 at the office</input>
<output>
{
  "success": true,
  "title": "Meeting with John",
  "description": "",
  "start": "15/09/2024 14:00",
  "end": "15/09/2024 15:00",
  "location": "office",
  "url": "",
  "attendee": ["John"]
}
</output>
</example>`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("<current-time>%s</current-time>\n<input>%s</input>", now, text),
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
		// return nil, fmt.Errorf("FromNaturalText: marshal req body: %w", err)
	}
	// #endregion

	// #region | send the request
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("FromNaturalText: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FromNaturalText: do request: %w", err)
	}
	// #endregion

	// #region | read the response
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FromNaturalText: bad status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("FromNaturalText: read body: %w", err)
	}
	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("FromNaturalText: unmarshal body: %w", err)
	}
	// #endregion

	// #region | assign the response content
	if len(respBody.Choices) == 0 {
		return nil, fmt.Errorf("FromNaturalText: no choices")
	}
	if len(respBody.Choices[0].Message.Content) == 0 {
		return nil, fmt.Errorf("FromNaturalText: no content")
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
		return nil, fmt.Errorf("FromNaturalText: unmarshal content: %w", err)
	}

	if !respContent.Success {
		return nil, fmt.Errorf("FromNaturalText: %s", respContent.Description)
	}

	// basic info
	e.ID = uuid.NewString()
	e.Summary = utils.CleanupString(respContent.Title)
	e.Description = utils.CleanupString(respContent.Description)

	// datetime related
	slog.Debug("FromNaturalText", "start", respContent.Start, "end", respContent.End)
	startDate, err := time.ParseInLocation("02/01/2006 15:04", respContent.Start, as.Config.GetLocation())
	if err != nil {
		return nil, fmt.Errorf("FromNaturalText: parse start date: %w", err)
	}
	e.StartDateUnixUTC = startDate.Unix()
	endDate, err := time.ParseInLocation("02/01/2006 15:04", respContent.End, as.Config.GetLocation())
	if err != nil {
		return nil, fmt.Errorf("FromNaturalText: parse end date: %w", err)
	}
	e.EndDateUnixUTC = endDate.Unix()

	// additional info
	e.Location = utils.CleanupString(respContent.Location)
	e.URL = respContent.URL
	// #endregion

	attendeeModels := make([]Attendee, len(respContent.Attendees))
	for i, attendee := range respContent.Attendees {
		attendeeModels[i] = Attendee{
			EventID: e.ID,
			Data:    attendee,
		}
	}

	return attendeeModels, nil
}
