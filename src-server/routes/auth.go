package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

type AuthRequestBody struct {
	UID  string `json:"uid"`
	TOTP string `json:"totp_code"`
}

func Auth(muxer *http.ServeMux, as *utils.AppState) {
	// logout
	muxer.HandleFunc("DELETE /auth", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "authorization=; Path=/; HttpOnly; SameSite=Lax")
		w.Header().Set("Set-Cookie", "uid=; Path=/; HttpOnly; SameSite=Lax")

		w.WriteHeader(http.StatusOK)
	})

	// login
	muxer.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		var reqBody AuthRequestBody
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Invalid request body"}`))
			return
		}
		if reqBody.TOTP == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Please provide a TOTP code"}`))
			return
		}
		if reqBody.UID == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Please provide a UID"}`))
			return
		}

		userModel := new(model.User)
		if err := as.BunDB.
			NewSelect().
			Model(userModel).
			Where("id = ?", reqBody.UID).
			Scan(context.Background()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"description": "User not found"}`))
			return
		}
		if !totp.Validate(reqBody.TOTP, userModel.TotpSecret) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"description": "Invalid TOTP code"}`))
			return
		}

		sessionSecret := uuid.NewString()
		sessionTokenModel := model.SessionToken{
			Secret:    sessionSecret,
			UserID:    reqBody.UID,
			CreatedAt: time.Now().Unix(),
			IpAddress: r.RemoteAddr,
			UserAgent: r.UserAgent(),
		}
		if _, err := as.BunDB.
			NewInsert().
			Model(sessionTokenModel).
			Exec(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"description": "Can't create session token"}`))
			return
		}

		w.Header().Set("Set-Cookie", "session-secret="+sessionSecret+"; Path=/; HttpOnly; SameSite=Lax")
		w.WriteHeader(http.StatusOK)
	})
}
