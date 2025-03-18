package utilities

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"inkwell-backend-V2.0/internal/model"
)

// Secret keys
var (
	accessSecret  = []byte("your-access-secret-key")
	refreshSecret = []byte("your-refresh-secret-key")
)

// Token expiration times
const (
	AccessTokenExpiry  = time.Minute * 15
	RefreshTokenExpiry = time.Hour * 24 * 7
)

// Claims struct
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateTokens creates both access and refresh tokens
func GenerateTokens(user *model.User) (string, string, error) {
	accessToken, err := generateToken(user, accessSecret, AccessTokenExpiry)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := generateToken(user, refreshSecret, RefreshTokenExpiry)
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
