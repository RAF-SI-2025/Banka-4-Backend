package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Pored standardnih polja (RegisteredClaims), dodajemo i UserID.
type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken kreira, potpisuje i vraća JWT token u obliku stringa.
func GenerateToken(userID uint, secret string, expiryMinutes int) (string, error) {

	expirationTime := time.Now().Add(time.Duration(expiryMinutes) * time.Minute)

	// Kreiramo payload (claims)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Kreiramo sam token koristeći HS256 algoritam za potpisivanje
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Potpisujemo token sa našom tajnom (secret) i pretvaramo ga u string
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
