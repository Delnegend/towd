package route

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func Auth(muxer *http.ServeMux, as *utils.AppState) {
	// logout
	muxer.HandleFunc("DELETE /auth", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "authorization=; Path=/; HttpOnly; SameSite=Lax")
		w.Header().Set("Set-Cookie", "uid=; Path=/; HttpOnly; SameSite=Lax")

		w.WriteHeader(http.StatusOK)
	})

	type AuthReqBody struct {
		TempKey string `json:"tempKey"`
	}

	// login
	muxer.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		// parse request body
		var reqBody AuthReqBody
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid request body"))
			return
		}

		newSessionSecret := uuid.NewString()
		allowThrough := false
		err := as.BunDB.RunInTx(r.Context(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			// check if tempKey exists in DB
			exists, err := as.BunDB.
				NewSelect().
				Model((*model.Session)(nil)).
				Where("secret = ?", reqBody.TempKey).
				Where("purpose = ?", model.SESSION_MODEL_PURPOSE_TEMP).
				Exists(r.Context())
			switch {
			case err != nil:
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't check if temp key exists in DB"))
				return fmt.Errorf("can't check if temp key exists in DB: %w", err)
			case !exists:
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Invalid temp key"))
				return nil
			}

			// get the sessionModel from tempKey from DB
			tempKeySessionModel := new(model.Session)
			if err := as.BunDB.
				NewSelect().
				Model(tempKeySessionModel).
				Where("secret = ?", reqBody.TempKey).
				Where("purpose = ?", model.SESSION_MODEL_PURPOSE_TEMP).
				Scan(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Can't find temp key in DB: %s", err.Error())))
				return fmt.Errorf("")
			}

			// delete the model from DB right away since it's one-time use
			if _, err := as.BunDB.
				NewDelete().
				Model((*model.Session)(nil)).
				Where("secret = ?", reqBody.TempKey).
				Where("purpose = ?", model.SESSION_MODEL_PURPOSE_TEMP).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Can't delete temp key in DB: %s", err.Error())))
				return fmt.Errorf("")
			}

			// check if tempKey is expired
			if time.Unix(tempKeySessionModel.CreatedAtUnixUTC, 0).UTC().
				Add(time.Minute * 5).Before(time.Now()) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Temp key expired"))
				return nil
			}

			// create new sessionModel for session
			if _, err := as.BunDB.
				NewInsert().
				Model(&model.Session{
					Secret:           newSessionSecret,
					Purpose:          model.SESSION_MODEL_PURPOSE_SESSION,
					UserID:           tempKeySessionModel.UserID,
					ChannelID:        tempKeySessionModel.ChannelID,
					CreatedAtUnixUTC: time.Now().UTC().Unix(),
				}).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Can't insert session model to DB: %s", err.Error())))
				return fmt.Errorf("")
			}
			allowThrough = true
			return nil
		})
		switch {
		case err != nil:
			return
		case !allowThrough:
			return
		}

		switch as.Config.GetDev() {
		case true:
			w.Write([]byte(fmt.Sprintf(`{"sessionSecret": "%s"}`, newSessionSecret)))
		case false:
			w.Header().Set("Set-Cookie", "session-secret="+newSessionSecret+"; Path=/; HttpOnly; SameSite=None")
		}
		w.WriteHeader(http.StatusOK)
	})
}
