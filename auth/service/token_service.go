package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenClaims struct {
	UserID  uuid.UUID `json:"user_id"`
	IsAdmin bool      `json:"is_admin"`
	jwt.RegisteredClaims
}

func (s *AuthService) GenerateToken(userPublicID uuid.UUID, isAdmin bool) (string, error) {
	claims := TokenClaims{
		UserID:  userPublicID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(
				time.Duration(s.config.JWTExpirationMinutes) * time.Minute,
			)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(s.config.JWTSecret), nil
		},
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}
