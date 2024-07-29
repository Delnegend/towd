package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"towd/src-server/jwt"
	"towd/src-server/model"
	"towd/src-server/utils"

	"github.com/pquerna/otp/totp"
)

type AuthRequestBody struct {
	Username string `json:"username"`
	JWTToken string `json:"jwt_token"`
	TotpCode string `json:"totp_code"`
}

type AuthResponseBody struct {
	Description string `json:"description,omitempty"`
	Token       string `json:"token,omitempty"`
}

func Auth(muxer *http.ServeMux, as *utils.AppState) {
	muxer.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		var reqBody AuthRequestBody
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Invalid request body"}`))
			return
		}

		switch {
		case reqBody.JWTToken != "":
			payload, err := jwt.Decode(reqBody.JWTToken, as.Config.GetJWTSecret())
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "Invalid JWT token"}`))
				return
			}

			expireAt := time.Unix(payload.IssuedAt+int64(as.Config.GetJWTExpire().Seconds()), 0)
			if time.Now().After(expireAt) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "Token expired"}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(payload)
		case reqBody.TotpCode != "":
			userModel := new(model.User)
			if err := as.BunDB.
				NewSelect().
				Model(userModel).
				Where("id = ?", reqBody.Username).
				Scan(context.Background()); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "User not found"}`))
				return
			}
			if !totp.Validate(reqBody.TotpCode, userModel.TotpSecret) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"description": "Invalid TOTP code"}`))
				return
			}

			token, err := jwt.Encode(jwt.Payload{
				UserID:   userModel.ID,
				UserName: userModel.Username,
				IssuedAt: time.Now().Unix(),
			}, as.Config.GetJWTSecret())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"description": "Can't generate token"}`))
				return
			}

			respBody := AuthResponseBody{
				Description: "Success",
				Token:       token,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(respBody)
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"description": "Please provide either a JWT token or a TOTP code"}`))
		}
	})
}
