package jwt

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func Encode(payload Payload, secret string) (string, error) {
	// paylooad
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("can't marshal payload: %v", err)
	}
	payloadBase64 := base64.StdEncoding.EncodeToString(payloadJson)

	// header
	header := struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}{
		Algorithm: "HS256",
		Type:      "JWT",
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("can't marshal header: %v", err)
	}
	headerBase64 := base64.StdEncoding.EncodeToString(headerJson)

	// signature
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(payloadBase64))
	sig := h.Sum(nil)
	sigBase64 := base64.StdEncoding.EncodeToString(sig)

	return fmt.Sprintf("%s.%s.%s", headerBase64, payloadBase64, sigBase64), nil
}
