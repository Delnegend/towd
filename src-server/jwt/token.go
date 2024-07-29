package jwt

type Token struct {
	Header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	} `json:"header"`
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}

type Payload struct {
	UserID   string `json:"id"`
	UserName string `json:"name"`
	IssuedAt int64  `json:"iat"`
}
