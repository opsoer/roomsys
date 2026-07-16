// 工具包，提供 JWT 令牌的生成、解析、吊销功能
package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 自定义声明，包含用户 ID、用户名、角色、楼宇 ID
type Claims struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	BuildingID uint   `json:"building_id"`
	jwt.RegisteredClaims
}

// revokedTokens 存储已吊销的令牌（键为令牌字符串，值为吊销时间）
var revokedTokens = make(map[string]time.Time)

// revokedTokensMu 保护 revokedTokens 的并发安全
var revokedTokensMu sync.RWMutex

// RevokeToken 将指定令牌加入吊销列表
func RevokeToken(tokenStr string) {
	revokedTokensMu.Lock()
	defer revokedTokensMu.Unlock()
	revokedTokens[tokenStr] = time.Now()
}

// IsTokenRevoked 检查指定令牌是否已被吊销
func IsTokenRevoked(tokenStr string) bool {
	revokedTokensMu.RLock()
	defer revokedTokensMu.RUnlock()
	_, revoked := revokedTokens[tokenStr]
	return revoked
}

// CleanupRevokedTokens 清理已过期超过 30 天的吊销记录
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

// GenerateToken 生成 JWT 访问令牌，有效期 72 小时
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

// GenerateRefreshToken 生成刷新令牌，有效期 720 小时（30 天）
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

// IsRefreshTokenFromClaims 根据 Claims 中的 ID 判断是否为刷新令牌
func IsRefreshTokenFromClaims(claims *Claims) bool {
	return claims.ID == "refresh"
}

// ParseToken 解析 JWT 令牌，校验签名并返回 Claims（会检查吊销状态）
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
