package route

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"
)

type SessionCtxKeyType string

const (
	SessionCtxKey           SessionCtxKeyType = "session"
	SessionSecretCookieName string            = "session-secret"
)

func AuthMiddleware(as *utils.AppState, next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// extract session secret from cookies
		sessionSecret := func() string {
			sessionCookie, err := r.Cookie(SessionSecretCookieName)
			if err == nil {
				return strings.TrimSpace(sessionCookie.Value)
			}
			return ""
		}()
		if sessionSecret == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Session secret cookie not found"))
			return
		}

		startTimer := time.Now()
		exists, err := as.BunDB.
			NewSelect().
			Model((*model.Session)(nil)).
			Where("secret = ?", sessionSecret).
			Where("purpose = ?", model.SESSION_MODEL_PURPOSE_SESSION).
			Exists(r.Context())
		switch {
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't check if session exists in DB"))
			slog.Error("can't check if session exists in DB", "error", err)
			return
		case !exists:
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Session secret not found"))
			return
		}
		as.MetricChans.DatabaseReadForAuthMiddleware <- float64(time.Since(startTimer).Microseconds())

		sessionModel := new(model.Session)
		if err := as.BunDB.
			NewSelect().
			Model(sessionModel).
			Where("secret = ?", sessionSecret).
			Where("purpose = ?", model.SESSION_MODEL_PURPOSE_SESSION).
			Scan(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Can't find session model in DB"))
			slog.Error("can't find session model in DB", "error", err)
			return
		}
		if time.Unix(sessionModel.CreatedAtUnixUTC, 0).UTC().
			Add(time.Hour * 24 * 7).Before(time.Now()) {
			if _, err := as.BunDB.
				NewDelete().
				Model((*model.Session)(nil)).
				Where("secret = ?", sessionSecret).
				Where("purpose = ?", model.SESSION_MODEL_PURPOSE_SESSION).
				Exec(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Can't delete session model in DB"))
				slog.Error("can't delete session model in DB", "error", err)
				return
			}

			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Session expired"))
			return
		}

		ctx := context.WithValue(r.Context(), SessionCtxKey, sessionModel)
		next(w, r.WithContext(ctx))
	}
}
