package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	SYSTEM_PROMPT = `This LLM system prompt handles JSON input and output for managing events. It supports creating, reading, updating, and deleting events based on user requests and event context, ensuring proper format and conditions are met.
` + "```" + `input
{"currentTime":string,"userRequest":string,"eventContext":{"title":string,"description":string,"start":string,"end":string,"location":string,"url":string,"attendees":list of strings}}
` + "```" + `
` + "```" + `output
{"success":bool,"action":"create" | "read" | "delete" | "update","description":string,"body":object}
` + "```" + `
` + "```" + `body
action == "create" or "update": {"title":string,"description":string,"start":string,"end":string,"location":string,"url":string,"attendees":list of strings}
action == "read": {"startDateToQuery":string,"endDateToQuery":string}
action == "delete": {}
` + "```" + `
` + "```" + `rules
- Datetime fields must be in DD/MM/YYYY HH:MM format.
- Create:
  + Must include title and start date. If similar to an existing event, eventContext must be provided.
  + If no end date, assume 1 hour duration.
- Read:
  + Require both startDateToQuery and endDateToQuery.
  + Success only if time-based. For queries like "Do I have a math exam tomorrow?", respond with "I don't have full database access, but here are the events for tomorrow (00:01 to 23:59)." + success: true.
- Update:
  + Require eventContext. If empty, set success to false with reason.
  + If a field is unchanged, copy from the old event instead of leaving it blank.
- Delete:
  + Check for event in event context (even if empty). If the request is outside the context, respond with "Sorry, you only have permission to delete the event in the event context." + success: false.
- Use natural, conversational language for errors. E.g., "You need to provide an event context" instead of "eventContext is required".
- Do not include questions or success messages in descriptions.
` + "```"
	GROQ_API   = "https://api.groq.com/openai/v1/chat/completions"
	GEMINI_API = "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s"
)

type LLMProvider string

const (
	LLMProviderGroq   LLMProvider = "groq"
	LLMProviderGemini LLMProvider = "gemini"
)

type Natural struct {
	provider LLMProvider
	req      *http.Request
}

func InitNatural(config *Config) (Natural, error) {
	var err error
	var natural Natural
	switch config.GetLLMProvider() {
	case LLMProviderGroq:
		if config.GetGroqApiKey() == "" {
			return Natural{}, fmt.Errorf("InitNatural: api key is blank")
		}
		natural.req, err = http.NewRequest("POST", GROQ_API, nil)
		natural.req.Header.Set("Authorization", " "+config.GetGroqApiKey())
	case LLMProviderGemini:
		if config.GetGeminiApiKey() == "" {
			return Natural{}, fmt.Errorf("InitNatural: api key is blank")
		}
		natural.req, err = http.NewRequest("POST", fmt.Sprintf(GEMINI_API, config.GetGeminiApiKey()), nil)
	default:
		return Natural{}, fmt.Errorf("InitNatural: unknown provider: %s", config.GetLLMProvider())
	}
	if err != nil {
		return Natural{}, fmt.Errorf("InitNatural: failed to create request: %w", err)
	}
	natural.provider = config.GetLLMProvider()
	natural.req.Header.Set("Content-Type", "application/json")
	return natural, nil
}

type (
	NaturalInput struct {
		CurrentTime  string                   `json:"currentTime"`
		UserRequest  string                   `json:"userRequest"`
		EventContext NaturalInputEventContext `json:"eventContext,omitempty"`
	}
	NaturalInputEventContext struct {
		Title       string   `json:"title"`
		Description string   `json:"description,omitempty"`
		Start       string   `json:"start"`
		End         string   `json:"end,omitempty"`
		Location    string   `json:"location,omitempty"`
		URL         string   `json:"url,omitempty"`
		Attendees   []string `json:"attendees,omitempty"`
	}

	NaturalOutput struct {
		Success     bool                    `json:"success"`
		Action      NaturalOutputActionType `json:"action"`
		Description string                  `json:"description"`
		Body        NaturalOutputBody       `json:"body"`
	}
	NaturalOutputActionType string
	NaturalOutputBody       struct {
		NaturalInputEventContext
		StartDateToQuery string `json:"startDateToQuery"`
		EndDateToQuery   string `json:"endDateToQuery"`
	}
)

const (
	NaturalOutputActionTypeCreate NaturalOutputActionType = "create"
	NaturalOutputActionTypeRead   NaturalOutputActionType = "read"
	NaturalOutputActionTypeUpdate NaturalOutputActionType = "update"
	NaturalOutputActionTypeDelete NaturalOutputActionType = "delete"
)

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
		return NaturalOutput{}, fmt.Errorf("(*Natural).NewRequest: failed to marshal input: %w", err)
	}

	switch n.provider {
	case LLMProviderGroq:
		return n.newGroqRequest(string(naturalInputBytes))
	case LLMProviderGemini:
		return n.newGeminiRequest(string(naturalInputBytes))
	default:
		return NaturalOutput{}, fmt.Errorf("(*Natural).NewRequest: unknown provider: %s", n.provider)
	}
}

func (n *Natural) newGroqRequest(text string) (NaturalOutput, error) {
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
				Role:    "system",
				Content: SYSTEM_PROMPT,
			},
			{
				Role:    "user",
				Content: text,
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
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: failed to marshal request body: %w", err)
	}

	// set req body and do request
	n.req.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
	resp, err := http.DefaultClient.Do(n.req)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: failed to do request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: failed to read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: [%d] %s", resp.StatusCode, string(body))
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
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: failed to unmarshal response: %w", err)
	}
	if len(respBody.Choices) == 0 {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: no choices")
	}
	if len(respBody.Choices[0].Message.Content) == 0 {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: no content")
	}
	respContent := NaturalOutput{}
	if err := json.Unmarshal([]byte(respBody.Choices[0].Message.Content), &respContent); err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGroqRequest: failed to unmarshal content: %w", err)
	}

	return respContent, nil
}

type (
	GeminiReqBodyContents struct {
		Role  string              `json:"role,omitempty"`
		Parts []GeminiReqBodyPart `json:"parts,omitempty"`
	}

	GeminiReqBodyPart struct {
		Text string `json:"text,omitempty"`
	}

	GeminiReqBodySystemInstruction struct {
		Role  string              `json:"role,omitempty"`
		Parts []GeminiReqBodyPart `json:"parts,omitempty"`
	}

	GeminiReqBodyGenerationConfig struct {
		Temperature      float64 `json:"temperature,omitempty"`
		TopK             int     `json:"topK,omitempty"`
		TopP             float64 `json:"topP,omitempty"`
		MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
		ResponseMimeType string  `json:"responseMimeType,omitempty"`
		// ResponseSchema   GeminiReqBodySchema `json:"responseSchema,omitempty"`
	}

	GeminiReqBodySchema struct {
		Type       string                  `json:"type,omitempty"`
		Properties GeminiReqBodyProperties `json:"properties,omitempty"`
		Required   []string                `json:"required,omitempty"`
	}

	GeminiReqBodyProperties struct {
		Success     GeminiReqBodyProperty `json:"success,omitempty"`
		Action      GeminiReqBodyProperty `json:"action,omitempty"`
		Description GeminiReqBodyProperty `json:"description,omitempty"`
		Body        GeminiReqBodyBody     `json:"body,omitempty"`
	}

	GeminiReqBodyProperty struct {
		Type string `json:"type,omitempty"`
	}

	GeminiReqBodyBody struct {
		Type       string                      `json:"type,omitempty"`
		Properties GeminiReqBodyBodyProperties `json:"properties,omitempty"`
	}

	GeminiReqBodyBodyProperties struct {
		Title            GeminiReqBodyProperty  `json:"title,omitempty"`
		Description      GeminiReqBodyProperty  `json:"description,omitempty"`
		Start            GeminiReqBodyProperty  `json:"start,omitempty"`
		End              GeminiReqBodyProperty  `json:"end,omitempty"`
		Location         GeminiReqBodyProperty  `json:"location,omitempty"`
		URL              GeminiReqBodyProperty  `json:"url,omitempty"`
		Attendees        GeminiReqBodyAttendees `json:"attendees,omitempty"`
		StartDateToQuery GeminiReqBodyProperty  `json:"startDateToQuery,omitempty"`
		EndDateToQuery   GeminiReqBodyProperty  `json:"endDateToQuery,omitempty"`
	}

	GeminiReqBodyAttendees struct {
		Type  string                `json:"type,omitempty"`
		Items GeminiReqBodyItemType `json:"items,omitempty"`
	}

	GeminiReqBodyItemType struct {
		Type string `json:"type,omitempty"`
	}

	// Main struct
	GeminiReqBody struct {
		Contents          []GeminiReqBodyContents        `json:"contents,omitempty"`
		SystemInstruction GeminiReqBodySystemInstruction `json:"systemInstruction,omitempty"`
		GenerationConfig  GeminiReqBodyGenerationConfig  `json:"generationConfig,omitempty"`
	}

	GeminiRespBody struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason  string `json:"finishReason"`
			Index         int    `json:"index"`
			SafetyRatings []struct {
				Category    string `json:"category"`
				Probability string `json:"probability"`
			} `json:"safetyRatings"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
)

func (n *Natural) newGeminiRequest(text string) (NaturalOutput, error) {
	// compose request body
	reqBody := GeminiReqBody{
		Contents: []GeminiReqBodyContents{
			{
				Role: "user",
				Parts: []GeminiReqBodyPart{
					{Text: text},
				},
			},
		},
		SystemInstruction: GeminiReqBodySystemInstruction{
			Role: "user",
			Parts: []GeminiReqBodyPart{
				{Text: SYSTEM_PROMPT},
			},
		},
		GenerationConfig: GeminiReqBodyGenerationConfig{
			Temperature:      1,
			TopK:             40,
			TopP:             0.95,
			MaxOutputTokens:  8192,
			ResponseMimeType: "application/json",
			// ResponseSchema: GeminiReqBodySchema{
			// 	Type: "object",
			// 	Properties: GeminiReqBodyProperties{
			// 		Success:     GeminiReqBodyProperty{Type: "boolean"},
			// 		Action:      GeminiReqBodyProperty{Type: "string"},
			// 		Description: GeminiReqBodyProperty{Type: "string"},
			// 		Body: GeminiReqBodyBody{
			// 			Type: "object",
			// 			Properties: GeminiReqBodyBodyProperties{
			// 				Title:       GeminiReqBodyProperty{Type: "string"},
			// 				Description: GeminiReqBodyProperty{Type: "string"},
			// 				Start:       GeminiReqBodyProperty{Type: "string"},
			// 				End:         GeminiReqBodyProperty{Type: "string"},
			// 				Location:    GeminiReqBodyProperty{Type: "string"},
			// 				URL:         GeminiReqBodyProperty{Type: "string"},
			// 				Attendees: GeminiReqBodyAttendees{
			// 					Type:  "array",
			// 					Items: GeminiReqBodyItemType{Type: "string"},
			// 				},
			// 				StartDateToQuery: GeminiReqBodyProperty{Type: "string"},
			// 				EndDateToQuery:   GeminiReqBodyProperty{Type: "string"},
			// 			},
			// 		},
			// 	},
			// 	Required: []string{"success", "action", "body"},
			// },
		},
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: failed to marshal request body: %w", err)
	}

	// set req body and do request
	n.req.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
	resp, err := http.DefaultClient.Do(n.req)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: failed to do request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: failed to read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: [%d] %s", resp.StatusCode, string(body))
	}

	// unmarshal response
	var respBody GeminiRespBody
	if err := json.Unmarshal(body, &respBody); err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: failed to unmarshal content: %w\n\n%s", err, string(body))
	}

	if len(respBody.Candidates) == 0 {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: no candidates")
	}
	if len(respBody.Candidates[0].Content.Parts) == 0 {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: no content")
	}
	respContent := NaturalOutput{}
	if err := json.Unmarshal([]byte(respBody.Candidates[0].Content.Parts[0].Text), &respContent); err != nil {
		return NaturalOutput{}, fmt.Errorf("(*Natural).newGeminiRequest: failed to unmarshal content: %w\n\n%s", err, string(body))
	}
	return respContent, nil
}
