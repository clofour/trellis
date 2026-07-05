package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func Initialize() string {
	b := make([]byte, 32)
	rand.Read(b)

	token := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))

	return token
}
