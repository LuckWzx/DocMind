package jwt

import (
	"errors"
	"time"

	"cloudque/pkg/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrTokenInvalid = errors.New("token 无效")
	ErrTokenExpired = errors.New("token 已过期")
)

// GenerateToken 生成 Access Token（24小时过期）
func GenerateToken(userID uint, username string) (string, error) {
	cfg := config.Get().JWT

	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.ExpireHours * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "docmind",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateRefreshToken 生成 Refresh Token（7天过期）
func GenerateRefreshToken(userID uint, username string) (string, error) {
	cfg := config.Get().JWT

	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "docmind",
			ID:        uuid.New().String(), // 使用 UUID 作为 Token ID
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken 解析 Access Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	cfg := config.Get().JWT

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// ParseRefreshToken 解析 Refresh Token
func ParseRefreshToken(tokenString string) (*CustomClaims, error) {
	// Refresh Token 和 Access Token 使用相同的解析逻辑
	return ParseToken(tokenString)
}

// RefreshToken 刷新 Token（已废弃，使用 AuthService.RefreshToken 代替）
func RefreshToken(tokenString string) (string, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	return GenerateToken(claims.UserID, claims.Username)
}
