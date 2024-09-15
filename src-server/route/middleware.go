package route

import (
	"context"
	"net/http"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"
)

type SessionCtxKeyType string

const SessionCtxKey SessionCtxKeyType = "session"

func AuthMiddleware(as *utils.AppState, next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
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
			w.Write([]byte(`{"description": "Session secret cookie not found"}`))
			return
		}

		session := new(model.Session)
		if err := as.BunDB.
			NewSelect().
			Model(session).
			Where("secret = ?", sessionSecret).
			Where("type = ?", "session").
			Scan(r.Context()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"description": "Session secret not found"}`))
			return
		}
		if session.CreatedAt.Add(time.Hour * 24 * 7).Before(time.Now()) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"description": "Session expired"}`))
			return
		}

		ctx := context.WithValue(r.Context(), SessionCtxKey, session)
		next(w, r.WithContext(ctx))
	}
}
