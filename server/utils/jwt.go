package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	BuildingID uint   `json:"building_id"`
	jwt.RegisteredClaims
}

var (
	revokedTokens   = make(map[string]time.Time)
	revokedTokensMu sync.RWMutex
)

func RevokeToken(tokenStr string) {
	revokedTokensMu.Lock()
	defer revokedTokensMu.Unlock()
	revokedTokens[tokenStr] = time.Now()
}

func IsTokenRevoked(tokenStr string) bool {
	revokedTokensMu.RLock()
	defer revokedTokensMu.RUnlock()
	_, revoked := revokedTokens[tokenStr]
	return revoked
}

func CleanupRevokedTokens() {
	revokedTokensMu.Lock()
	defer revokedTokensMu.Unlock()
	now := time.Now()
	for token, t := range revokedTokens {
		if now.Sub(t) > 720*time.Hour {
			delete(revokedTokens, token)
		}
	}
}

func GenerateToken(userID uint, username, role, secret string, buildingID uint) (string, error) {
	claims := Claims{
		UserID:     userID,
		Username:   username,
		Role:       role,
		BuildingID: buildingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GenerateRefreshToken(userID uint, username, role, secret string, buildingID uint) (string, error) {
	claims := Claims{
		UserID:     userID,
		Username:   username,
		Role:       role,
		BuildingID: buildingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(Now().Add(720 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(Now()),
			ID:        "refresh",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func IsRefreshTokenFromClaims(claims *Claims) bool {
	return claims.ID == "refresh"
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
	if IsTokenRevoked(tokenStr) {
		return nil, fmt.Errorf("token has been revoked")
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}
