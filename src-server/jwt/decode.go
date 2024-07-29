package jwt

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

func Decode(token string, secret string) (*Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token length")
	}

	// payload
	payloadBase64 := parts[1]
	payloadJson, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, fmt.Errorf("can't decode payload: %v", err)
	}
	var payload Payload
	err = json.Unmarshal(payloadJson, &payload)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal payload: %v", err)
	}

	// signature
	signatureBase64 := parts[2]
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return nil, fmt.Errorf("can't decode signature: %v", err)
	}
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(payloadBase64))

	// validate signature
	if !hmac.Equal(h.Sum(nil), []byte(signature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	return &payload, nil
}
