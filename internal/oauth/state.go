package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// RandomState returns a random OAuth state value for CSRF protection.
func RandomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
