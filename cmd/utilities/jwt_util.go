package utilities

import (
	"errors"
	"fmt"
	"inkwell-backend-V2.0/cmd/internal/config"
	"inkwell-backend-V2.0/cmd/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Secret keys and expiration times from config
var (
	accessSecret  []byte
	refreshSecret []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
)

// InitAuthConfig Initialize config values once
func InitAuthConfig(cfg *config.APIConfig) {
	if cfg == nil {
		panic("failed to load configuration")
	}
	// Load secrets
	accessSecret = []byte(cfg.Authentication.SecretKeys["ACCESS"])
	refreshSecret = []byte(cfg.Authentication.SecretKeys["REFRESH"])

	// Load expiration times based on time units
	accessExpiry = parseDuration(cfg.Authentication.SessionTimeouts["ACCESS"], cfg.Authentication.TimeUnits["ACCESS"])
	refreshExpiry = parseDuration(cfg.Authentication.SessionTimeouts["REFRESH"], cfg.Authentication.TimeUnits["REFRESH"])
}

// parseDuration converts session timeout values based on the provided time unit
func parseDuration(value int, unit string) time.Duration {
	switch unit {
	case "SECONDS":
		return time.Duration(value) * time.Second
	case "MINUTES":
		return time.Duration(value) * time.Minute
	case "HOURS":
		return time.Duration(value) * time.Hour
	default:
		fmt.Printf("Warning: Unknown time unit '%s', defaulting to SECONDS\n", unit)
		return time.Duration(value) * time.Second
	}
}

// Claims struct
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateTokens creates both access and refresh tokens
func GenerateTokens(user *model.User) (string, string, error) {
	accessToken, err := generateToken(user, accessSecret, accessExpiry)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := generateToken(user, refreshSecret, refreshExpiry)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken verifies the token and extracts claims
func ValidateToken(tokenStr string, isRefresh bool) (*Claims, error) {
	secret := accessSecret
	if isRefresh {
		secret = refreshSecret
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})

	if err != nil {
		return nil, errors.New("invalid or malformed token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Explicitly check expiration
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token has expired")
	}

	return claims, nil
}

// RefreshTokens generates a new access and refresh token using a valid refresh token
func RefreshTokens(refreshToken string) (string, string, error) {
	claims, err := ValidateToken(refreshToken, true)
	if err != nil {
		return "", "", errors.New("invalid or expired refresh token")
	}

	// Explicitly check if the refresh token is expired
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return "", "", errors.New("refresh token has expired")
	}

	// Generate new tokens
	newAccessToken, newRefreshToken, err := GenerateTokens(&model.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
	})
	if err != nil {
		return "", "", errors.New("failed to generate new tokens")
	}

	return newAccessToken, newRefreshToken, nil
}

// Helper function to generate JWT token
func generateToken(user *model.User, secret []byte, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.Email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
