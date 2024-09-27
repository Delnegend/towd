package route

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"towd/src-server/model"
	"towd/src-server/utils"
)

func Kanban(muxer *http.ServeMux, as *utils.AppState) {
	type KanbanItemReqRespBody struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	}

	type KanbanGroupReqRespBody struct {
		Name  string                  `json:"groupName"`
		Items []KanbanItemReqRespBody `json:"items"`
	}

	type KanbanTableReqRespBody struct {
		TableName string                   `json:"tableName"`
		Groups    []KanbanGroupReqRespBody `json:"groups"`
	}

	// get the entire kanban table
	muxer.HandleFunc("GET /kanban/load", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		// check if exists, if not create a new one
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.KanbanTable)(nil)).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exists(r.Context())
		switch {
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't check if Kanban table exists: %s", err.Error())))
			return
		case !exists:
			kanbanTableModel := model.KanbanTable{
				ChannelID: sessionModel.ChannelID,
				Name:      "Untitled",
			}
			if _, err := as.BunDB.NewInsert().
				Model(&kanbanTableModel).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Can't insert Kanban table: %s", err.Error())))
				return
			}
		}

		kanbanTableModel := new(model.KanbanTable)
		if err := as.BunDB.
			NewSelect().
			Model(kanbanTableModel).
			Where("channel_id = ?", sessionModel.ChannelID).
			Scan(r.Context(), kanbanTableModel); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't get Kanban table: %s", err.Error())))
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
			w.Write([]byte(fmt.Sprintf("Can't get Kanban groups: %s", err.Error())))
			return
		}

		resp := KanbanTableReqRespBody{
			TableName: kanbanTableModel.Name,
		}
		for _, group := range groups {
			respGroup := KanbanGroupReqRespBody{
				Name:  group.Name,
				Items: make([]KanbanItemReqRespBody, 0),
			}
			for _, item := range group.Items {
				respItem := KanbanItemReqRespBody{
					ID:      item.ID,
					Content: item.Content,
				}
				respGroup.Items = append(respGroup.Items, respItem)
			}
			resp.Groups = append(resp.Groups, respGroup)
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't encode Kanban response: %s", err.Error())))
			return
		}
	}))

	// overwrite the entire kanban table
	muxer.HandleFunc("POST /kanban/save", AuthMiddleware(as, func(w http.ResponseWriter, r *http.Request) {
		sessionModel, ok := r.Context().Value(SessionCtxKey).(*model.Session)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't get session from middleware"))
			return
		}

		// parse request body
		var reqBody KanbanTableReqRespBody
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}

		// remove the kanban table from the database
		if _, err := as.BunDB.NewDelete().
			Model((*model.KanbanTable)(nil)).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't delete Kanban table: %s", err.Error())))
			return
		}
		if _, err := as.BunDB.NewDelete().
			Model((*model.KanbanGroup)(nil)).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't delete Kanban groups: %s", err.Error())))
			return
		}
		if _, err := as.BunDB.NewDelete().
			Model((*model.KanbanItem)(nil)).
			Where("channel_id = ?", sessionModel.ChannelID).
			Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't delete Kanban items: %s", err.Error())))
			return
		}

		// insert the new kanban table, groups and items into the database
		if _, err := as.BunDB.NewInsert().
			Model(&model.KanbanTable{
				Name:      reqBody.TableName,
				ChannelID: sessionModel.ChannelID,
			}).
			Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Can't insert Kanban table: %s", err.Error())))
			return
		}

		if len(reqBody.Groups) > 0 {
			kanbanGroupModels := make([]model.KanbanGroup, 0)
			for _, group := range reqBody.Groups {
				kanbanGroupModels = append(kanbanGroupModels, model.KanbanGroup{
					Name:      group.Name,
					ChannelID: sessionModel.ChannelID,
				})
			}
			if _, err := as.BunDB.NewInsert().
				Model(&kanbanGroupModels).
				Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Can't insert Kanban groups: %s", err.Error())))
				return
			}
			kanbanItemModels := make([]model.KanbanItem, 0)
			for _, group := range reqBody.Groups {
				for _, item := range group.Items {
					kanbanItemModels = append(kanbanItemModels, model.KanbanItem{
						ID:        item.ID,
						Content:   item.Content,
						GroupName: group.Name,
						ChannelID: sessionModel.ChannelID,
					})
				}
			}
			if len(kanbanItemModels) > 0 {
				if _, err := as.BunDB.NewInsert().
					Model(&kanbanItemModels).
					Exec(context.WithValue(r.Context(), model.KanbanBoardChannelIDCtxKey, sessionModel.ChannelID)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(fmt.Sprintf("Can't insert Kanban items: %s", err.Error())))
					return
				}
			}
		}

		w.WriteHeader(http.StatusOK)
	}))
}
