package routes

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"
)

type UserModelCtxKeyType string

const UserModelCtxKey UserModelCtxKeyType = "user-model"

func AuthMiddleware(as *utils.AppState, orig func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// extract session secret
		sessionSecret := func() string {
			sessionCookie, err := r.Cookie("session-secret")
			if err == nil {
				return strings.TrimSpace(sessionCookie.Value)
			}
			return ""
		}()
		if sessionSecret == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// get session token model
		sessionTokenModel := new(model.SessionToken)
		if err := as.BunDB.
			NewSelect().
			Model(sessionTokenModel).
			Where("secret = ?", sessionSecret).
			Scan(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"description": "Can't find session token to validate session secret"}`))
			return
		}

		// invalidate if older than 1 week
		createdAt := time.Unix(sessionTokenModel.CreatedAtUnix, 0).UTC()
		if time.Now().UTC().Sub(createdAt) > time.Hour*24*7 {
			if _, err := as.BunDB.NewDelete().
				Model((*model.SessionToken)(nil)).
				Where("secret = ?", sessionSecret).
				Exec(context.Background()); err != nil {
				slog.Warn("can't delete session token", "where", "routes/middleware.go", "err", err)
			}

			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		userModel := new(model.User)
		if err := as.BunDB.
			NewSelect().
			Model(userModel).
			Where("id = ?", sessionTokenModel.UserID).
			Scan(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"description": "Can't find user to validate session token"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserModelCtxKey, userModel)
		orig(w, r.WithContext(ctx))
	}
}
