package auth

import "common/pkg/jwt"

// TokenVerifier validates a raw JWT string and returns parsed claims.
// The standard implementation is jwt.JWTVerifier which validates locally
// using a shared HMAC secret.
type TokenVerifier interface {
	VerifyToken(tokenString string) (*jwt.Claims, error)
}
