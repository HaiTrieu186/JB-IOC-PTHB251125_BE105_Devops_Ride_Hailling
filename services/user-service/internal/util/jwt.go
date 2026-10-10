package util

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenClaims struct {
	UserID string  `json:"user_id"`
	Role   string  `json:"role"`
	Exp    int64   `json:"exp"`
	JTI    string  `json:"jti"`
}

func GenerateToken(userID string, role string, expirySeconds int, secret string) (string, string, int64, error) {
	jti := uuid.New().String()
	now := time.Now().UTC()
	exp := now.Add(time.Duration(expirySeconds) * time.Second).Unix()

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     exp,
		"jti":     jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, exp, nil
}

func ValidateToken(tokenString string, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	return claims, nil
}
