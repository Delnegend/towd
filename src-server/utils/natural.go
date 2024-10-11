package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Natural struct {
	req *http.Request
}

func InitNatural(apiKey string) (Natural, error) {
	if apiKey == "" {
		return Natural{}, fmt.Errorf("InitNatural: api key is blank")
	}
	var err error
	var natural Natural
	natural.req, err = http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", nil)
	if err != nil {
		return Natural{}, fmt.Errorf("InitNatural: failed to create request: %w", err)
	}
	natural.req.Header.Set("Content-Type", "application/json")
	natural.req.Header.Set("Authorization", "Bearer "+apiKey)
	return natural, nil
}

type NaturalInputEventContext struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Start       string   `json:"start"`
	End         string   `json:"end,omitempty"`
	Location    string   `json:"location,omitempty"`
	URL         string   `json:"url,omitempty"`
	Attendees   []string `json:"attendees,omitempty"`
}

type NaturalInput struct {
	CurrentTime  string                   `json:"current_time"`
	UserRequest  string                   `json:"user_request"`
	EventContext NaturalInputEventContext `json:"event_context,omitempty"`
}

type (
	NaturalOutputBody       interface{}
	NaturalOutputActionType string
)

const (
	NaturalOutputActionTypeCreate NaturalOutputActionType = "create"
	NaturalOutputActionTypeRead   NaturalOutputActionType = "read"
	NaturalOutputActionTypeUpdate NaturalOutputActionType = "update"
	NaturalOutputActionTypeDelete NaturalOutputActionType = "delete"
)

type NaturalOutputBodyForCreateOrUpdate NaturalInputEventContext

type NaturalOutputBodyForRead struct {
	StartDateToQuery string `json:"start_date_to_query"`
	EndDateToQuery   string `json:"end_date_to_query"`
}

type NaturalOutputBodyForDelete struct{}

type NaturalOutput struct {
	Success     bool                    `json:"success"`
	Action      NaturalOutputActionType `json:"action"`
	Description string                  `json:"description"`
	Body        NaturalOutputBody       `json:"body"`
}

func (n *Natural) NewRequest(text string, eventContext *NaturalInputEventContext) (NaturalOutput, error) {
	if text == "" {
		return NaturalOutput{}, fmt.Errorf("(*Natural).NewRequest: text is blank")
	}

	// compose new input
	now := time.Now().UTC().Truncate(24 * time.Hour).Format("02/01/2006 15:04")
	naturalInput := NaturalInput{
		CurrentTime: now,
		UserRequest: text,
	}
	if eventContext != nil {
		naturalInput.EventContext = *eventContext
	}
	naturalInputBytes, err := json.Marshal(naturalInput)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("NewRequest: failed to marshal input: %w", err)
	}

	// compose request body
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
				Content: `This LLM system prompt handles JSON input and output for managing events. It supports creating, reading, updating, and deleting events based on user requests and event context, ensuring proper format and conditions are met.
` + "```" + `input
{"current_time":string,"user_request":string,"event_context":{"title":string,"description":string,"start":string,"end":string,"location":string,"url":string,"attendees":list of strings}}
` + "```" + `
` + "```" + `output
{"success":bool,"action":"create" | "read" | "delete" | "update","description":string,"body":object}
` + "```" + `
` + "```" + `body
action == "create" or "update": {"title":string,"description":string,"start":string,"end":string,"location":string,"url":string,"attendees":list of strings}
action == "read": {"start_date_to_query":string,"end_date_to_query":string}
action == "delete": {}
` + "```" + `
` + "```" + `rules
- Datetime fields must be in DD/MM/YYYY HH:MM format.
- Create:
  + Must include title and start date. If similar to an existing event, event_context must be provided.
  + If no end date, assume 1 hour duration.
- Read:
  + Require both start_date_to_query and end_date_to_query.
  + Success only if time-based. For queries like "Do I have a math exam tomorrow?", respond with "I don't have full database access, but here are the events for tomorrow (00:01 to 23:59)." + success: true.
- Update:
  + Require event_context. If empty, set success to false with reason.
- Delete:
  + Check for event in event context (even if empty). If the request is outside the context, respond with "Sorry, you only have permission to delete the event in the event context." + success: false.
- Use natural, conversational language for errors. E.g., "You need to provide an event context" instead of "event_context is required".
- Do not include questions or success messages in descriptions.
` + "```",
			},
			{
				Role:    "user",
				Content: string(naturalInputBytes),
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
		return NaturalOutput{}, fmt.Errorf("InitNatural: failed to marshal request body: %w", err)
	}

	// set req body and do request
	n.req.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
	resp, err := http.DefaultClient.Do(n.req)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("InitNatural: failed to do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NaturalOutput{}, fmt.Errorf("InitNatural: bad status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("InitNatural: failed to read body: %w", err)
	}

	// unmarshal response
	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &respBody); err != nil {
		return NaturalOutput{}, fmt.Errorf("InitNatural: failed to unmarshal response: %w", err)
	}
	if len(respBody.Choices) == 0 {
		return NaturalOutput{}, fmt.Errorf("InitNatural: no choices")
	}
	if len(respBody.Choices[0].Message.Content) == 0 {
		return NaturalOutput{}, fmt.Errorf("InitNatural: no content")
	}
	respContent := NaturalOutput{}
	if err := json.Unmarshal([]byte(respBody.Choices[0].Message.Content), &respContent); err != nil {
		return NaturalOutput{}, fmt.Errorf("InitNatural: failed to unmarshal content: %w", err)
	}

	return respContent, nil
}
