package util

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UserID string
	Role   string
	JTI    string
	Exp    int64
}

func ValidateToken(tokenString, secret string) (*TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	userID, _ := claims["user_id"].(string)
	role, _ := claims["role"].(string)
	jti, _ := claims["jti"].(string)

	if userID == "" || role == "" || jti == "" {
		return nil, fmt.Errorf("missing required claims in token")
	}

	var exp int64
	switch v := claims["exp"].(type) {
	case float64:
		exp = int64(v)
	case int64:
		exp = v
	default:
		return nil, fmt.Errorf("invalid exp claim type")
	}

	return &TokenClaims{
		UserID: userID,
		Role:   role,
		JTI:    jti,
		Exp:    exp,
	}, nil
}
