package route

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/google/uuid"
)

func Calendar(muxer *http.ServeMux, as *utils.AppState) {
	type GetEventsReqBody struct {
		StartDateUnixUTC int64 `json:"startDateUnixUTC"`
		EndDateUnixUTC   int64 `json:"endDateUnixUTC"`
	}

	type OneEventRespBody struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		Location         string `json:"location"`
		Url              string `json:"url"`
		Organizer        string `json:"organizer"`
		StartDateUnixUTC int64  `json:"startDateUnixUTC"`
		EndDateUnixUTC   int64  `json:"endDateUnixUTC"`
		IsWholeDay       bool   `json:"isWholeDay"`
	}

	// get all events in date range
	muxer.HandleFunc("POST /calendar/get-events", AuthMiddleware(as,
		func(w http.ResponseWriter, r *http.Request) {
			sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't get session from middleware"))
				return
			}

			// #region - parse date
			var reqBody GetEventsReqBody
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid request body"))
				return
			}
			if reqBody.StartDateUnixUTC == 0 || reqBody.EndDateUnixUTC == 0 {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Please provide a start date and end date"))
				return
			}
			startDate := time.Unix(reqBody.StartDateUnixUTC, 0).UTC()
			endDate := time.Unix(reqBody.EndDateUnixUTC, 0).UTC()
			// #endregion

			// #region - get all events & prepare response body
			eventModels := make([]model.Event, 0)
			if err := as.BunDB.
				NewSelect().
				Model(&eventModels).
				Where("channel_id = ?", sessionModel.ChannelID).
				Where("start_date >= ?", startDate.Unix()).
				Where("end_date <= ?", endDate.Unix()).
				Relation("Attendees").
				Scan(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't get events"))
				return
			}

			respBody := make([]OneEventRespBody, 0)
			for _, event := range eventModels {
				respBody = append(respBody, OneEventRespBody{
					ID:               event.ID,
					Title:            event.Summary,
					Description:      event.Description,
					Location:         event.Location,
					Url:              event.URL,
					Organizer:        event.Organizer,
					StartDateUnixUTC: event.StartDateUnixUTC,
					EndDateUnixUTC:   event.EndDateUnixUTC,
					IsWholeDay:       event.IsWholeDay,
				})
			}
			respBodyJson, err := json.Marshal(respBody)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't marshal response body"))
				return
			}
			// #endregion

			w.WriteHeader(http.StatusOK)
			w.Write(respBodyJson)
		}))

	type CreateEventReqBody struct {
		Title            string `json:"title"`
		Description      string `json:"description"`
		Location         string `json:"location"`
		Url              string `json:"url"`
		Organizer        string `json:"organizer"`
		StartDateUnixUTC int64  `json:"startDateUnixUTC"`
		EndDateUnixUTC   int64  `json:"endDateUnixUTC"`
	}

	type ModifyEventReqBody struct {
		ID string `json:"id"`
		CreateEventReqBody
	}

	// create a new event, the success response is the event ID
	muxer.HandleFunc("POST /calendar/create-event", AuthMiddleware(as,
		func(w http.ResponseWriter, r *http.Request) {
			sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't get session from middleware"))
				return
			}

			// parse request body
			var reqBody CreateEventReqBody
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid request body"))
				return
			}

			// ensure calendar exists
			exists, err := as.BunDB.
				NewSelect().
				Model((*model.Calendar)(nil)).
				Where("channel_id = ?", sessionModel.ChannelID).
				Exists(r.Context())
			switch {
			case err != nil:
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't check if calendar exists"))
				return
			case !exists:
				calendarName := func() string {
					channels, err := as.DgSession.GuildChannels(sessionModel.ChannelID)
					if err != nil {
						slog.Warn("can't get channel name to create calendar", "where", "route/calendar.go", "error", err)
						return ""
					}
					var channelName string
					for _, channel := range channels {
						if channel.ID == sessionModel.ChannelID {
							channelName = channel.Name
							break
						}
					}
					if channelName == "" {
						channelName = "Untitled"
					}
					return channelName
				}()
				if _, err := as.BunDB.NewInsert().
					Model(&model.Calendar{
						ChannelID: sessionModel.ChannelID,
						Name:      calendarName,
					}).
					Exec(r.Context()); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Can't create calendar"))
					return
				}
			}

			// create event
			newEvent := model.Event{
				ID:               uuid.NewString(),
				Summary:          reqBody.Title,
				Description:      reqBody.Description,
				Location:         reqBody.Location,
				URL:              reqBody.Url,
				Organizer:        reqBody.Organizer,
				StartDateUnixUTC: reqBody.StartDateUnixUTC,
				EndDateUnixUTC:   reqBody.EndDateUnixUTC,
				CalendarID:       sessionModel.ChannelID,
				ChannelID:        sessionModel.ChannelID,
			}
			if newEvent.Upsert(r.Context(), as.BunDB) != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't create event"))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(newEvent.ID))
		}))

	// modify an existing event
	muxer.HandleFunc("POST /calendar/modify-event", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		// parse request body
		var reqBody ModifyEventReqBody
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid request body"))
			return
		}

		// check if event exists
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.Event)(nil)).
			Where("id = ?", reqBody.ID).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exists(r.Context())
		switch {
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't check if event exists"))
			return
		case !exists:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Event not found"))
			return
		}

		eventModel := new(model.Event)
		if err := as.BunDB.
			NewSelect().
			Model(eventModel).
			Where("id = ?", reqBody.ID).
			Where("channel_id = ?", sessionModel.ChannelID).
			Scan(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get event"))
			return
		}

		eventModel.Summary = reqBody.Title
		eventModel.Description = reqBody.Description
		eventModel.Location = reqBody.Location
		eventModel.URL = reqBody.Url
		eventModel.Organizer = reqBody.Organizer
		eventModel.StartDateUnixUTC = reqBody.StartDateUnixUTC
		eventModel.EndDateUnixUTC = reqBody.EndDateUnixUTC

		// modify event
		if err := eventModel.Upsert(r.Context(), as.BunDB); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(reqBody.ID))
	}))

	// delete an event
	muxer.HandleFunc("DELETE /event/{id}", func(w http.ResponseWriter, r *http.Request) {
		sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		id := r.PathValue("id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Please provide an event ID"))
			return
		}

		// delete the event
		if _, err := as.BunDB.NewDelete().
			Model((*model.Event)(nil)).
			Where("id = ?", id).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exec(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't delete event"))
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
