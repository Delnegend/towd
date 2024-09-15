package routes

import (
	"context"
	"database/sql"
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

	// login
	newSessionSecret := uuid.NewString()
	muxer.HandleFunc("POST /auth/:tempKey", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("tempKey") == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Please provide a temp key"}`))
			return
		}

		err := as.BunDB.RunInTx(r.Context(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			exists, err := as.BunDB.
				NewSelect().
				Model((*model.Session)(nil)).
				Where("secret = ?", r.PathValue("tempKey")).
				Where("type = ?", "temp").
				Exists(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"description": "Can't find temp key"}`))
				return fmt.Errorf("can't find temp key: %w", err)
			}
			if !exists {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "Invalid temp key"}`))
				return fmt.Errorf("invalid temp key")
			}

			tempKeyInDatabase := new(model.Session)
			if err := as.BunDB.
				NewSelect().
				Model(tempKeyInDatabase).
				Where("secret = ?", r.PathValue("tempKey")).
				Scan(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"description": "Can't find temp key"}`))
				return fmt.Errorf("can't find temp key: %w", err)
			}

			if tempKeyInDatabase.CreatedAt.Add(time.Minute * 5).Before(time.Now()) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "Temp key expired"}`))
				return fmt.Errorf("temp key expired")
			}

			if _, err := as.BunDB.
				NewDelete().
				Model((*model.Session)(nil)).
				Where("secret = ?", r.PathValue("tempKey")).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"description": "Can't delete temp key"}`))
				return fmt.Errorf("can't delete temp key: %w", err)
			}

			newSession := model.Session{
				Secret:    newSessionSecret,
				Type:      "session",
				UserID:    tempKeyInDatabase.UserID,
				ChannelID: tempKeyInDatabase.ChannelID,
				CreatedAt: time.Now(),
			}
			if _, err := as.BunDB.
				NewInsert().
				Model(&newSession).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"description": "Can't create session"}`))
				return fmt.Errorf("can't create session: %w", err)
			}
			return nil
		})
		if err != nil {
			return
		}

		w.Header().Set("Set-Cookie", "session-secret="+newSessionSecret+"; Path=/; HttpOnly; SameSite=Lax")
		w.WriteHeader(http.StatusOK)
	})
}
