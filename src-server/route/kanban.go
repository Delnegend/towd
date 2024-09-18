package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"towd/src-server/model"
	"towd/src-server/utils"
)

func Kanban(muxer *http.ServeMux, as *utils.AppState) {
	type KanbanItemRespBody struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	}

	type KanbanGroupRespBody struct {
		Name  string               `json:"groupName"`
		Items []KanbanItemRespBody `json:"items"`
	}

	type KanbanTableRespBody struct {
		TableName string                `json:"tableName"`
		Groups    []KanbanGroupRespBody `json:"groups"`
	}

	muxer.HandleFunc("GET /kanban/get-groups", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		sessionModel, ok := r.Context().Value(SessionCtxKey).(model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		kanbanTableModel := new(model.KanbanTable)
		if err := as.BunDB.
			NewSelect().
			Model(kanbanTableModel).
			Where("channel_id = ?", sessionModel.ChannelID).
			Scan(r.Context(), kanbanTableModel); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get Kanban table"))
			return
		}

		groups := make([]model.KanbanGroup, 0)
		if err := as.BunDB.
			NewSelect().
			Model(&groups).
			Where("channel_id = ?", sessionModel.ChannelID).
			Relation("Items").
			Scan(r.Context(), &groups); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get Kanban groups"))
			return
		}

		resp := KanbanTableRespBody{
			TableName: kanbanTableModel.Name,
		}
		for _, group := range groups {
			respGroup := KanbanGroupRespBody{
				Name:  group.Name,
				Items: make([]KanbanItemRespBody, 0),
			}
			for _, item := range group.Items {
				respItem := KanbanItemRespBody{
					ID:      item.ID,
					Content: item.Content,
				}
				respGroup.Items = append(respGroup.Items, respItem)
			}
			resp.Groups = append(resp.Groups, respGroup)
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't encode Kanban response"))
			return
		}
	}))

	type CreateKanbanItemReqBody struct {
		GroupName string `json:"groupName"`
		Content   string `json:"content"`
	}

	// create a new kanban item and return its ID
	muxer.HandleFunc("POST /kanban/create-item", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		// get session model from context
		sessionModel, ok := r.Context().Value(SessionCtxKey).(model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		// parse request body
		var reqBody CreateKanbanItemReqBody
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Can't decode request body"))
			return
		}

		// check if group name exists
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.KanbanTable)(nil)).
			Where("name = ?", reqBody.GroupName).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exists(r.Context())
		switch {
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't check if Kanban table exists"))
			return
		case !exists:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Kanban table not found"))
			return
		}

		// insert new kanban item & get its ID
		res, err := as.BunDB.NewInsert().
			Model(&model.KanbanItem{
				Content:   reqBody.Content,
				GroupName: reqBody.GroupName,
				ChannelID: sessionModel.ChannelID,
			}).
			Exec(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't insert Kanban item"))
			return
		}
		itemID, err := res.LastInsertId()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get last inserted Kanban item ID"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%d", itemID)))
	}))

	muxer.HandleFunc("DELETE /kanban/delete-item/:id", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		// get session model from context
		sessionModel, ok := r.Context().Value(SessionCtxKey).(model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		eventID := r.PathValue("id")
		if eventID == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Please provide an event ID"))
			return
		}

		// delete the event
		if _, err := as.BunDB.NewDelete().
			Model((*model.KanbanItem)(nil)).
			Where("id = ?", eventID).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exec(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't delete event"))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
}
