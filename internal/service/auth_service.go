package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"inkwell-backend-V2.0/utilities"

	"golang.org/x/crypto/bcrypt"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

// AuthService interface
type AuthService interface {
	Register(user *model.User) error
	Login(username, authhash string) (*LoginResponse, error)
	RefreshTokens(refreshToken string) (*TokenResponse, error)
}

type authService struct {
	userRepo repository.UserRepository
}

// NewAuthService initializes authentication service
func NewAuthService(userRepo repository.UserRepository) AuthService {
	return &authService{userRepo: userRepo}
}

// hash256encode hashes a password using SHA-256
func hash256encode(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (s *authService) Register(user *model.User) error {
	existingUser, err := s.userRepo.GetUserByEmail(user.Email)
	if err == nil && existingUser != nil {
		return errors.New("email already in use")
	}

	if user.Password == "" {
		return errors.New("password cannot be empty")
	}

	// First, apply SHA-256 hashing
	hashedPassword := hash256encode(user.Password)

	// Store only the SHA-256 hash
	user.Password = hashedPassword

	// Save user to DB
	err = s.userRepo.CreateUser(user)
	if err != nil {
		return errors.New("failed to create user")
	}

	return nil
}

// LoginResponse struct
type LoginResponse struct {
	User    *model.User `json:"user"`
	Access  string      `json:"access"`
	Refresh string      `json:"refresh"`
}

// Login function to authenticate user
func (s *authService) Login(username, authhash string) (*LoginResponse, error) {
	// Step 1: Retrieve user from database (already stored as SHA-256 hash)
	user, err := s.userRepo.GetUserByEmail(username)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Step 2: Concatenate email with stored SHA-256 hashed password
	concatenatedString := username + "::" + user.Password

	// Step 3: Decode Base64 `authhash` received from frontend
	bcryptEncryptedBytes, err := base64.StdEncoding.DecodeString(authhash)
	if err != nil {
		return nil, errors.New("invalid authhash format")
	}
	bcryptEncrypted := string(bcryptEncryptedBytes)

	// Step 4: Compare bcrypt hash with concatenated string
	err = bcrypt.CompareHashAndPassword([]byte(bcryptEncrypted), []byte(concatenatedString))
	if err != nil {
		fmt.Println("Bcrypt Comparison Failed:", err)
		return nil, errors.New("invalid credentials")
	}

	// Step 5: Remove password before returning user data
	user.Password = ""

	// Step 6: Generate access and refresh tokens
	accessToken, refreshToken, err := utilities.GenerateTokens(user)
	if err != nil {
		return nil, errors.New("failed to generate tokens")
	}

	// Step 7: Return response in expected format
	return &LoginResponse{
		User:    user,
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

// TokenResponse struct for refresh tokens
type TokenResponse struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}

// RefreshTokens function to refresh both access and refresh tokens
func (s *authService) RefreshTokens(refreshToken string) (*TokenResponse, error) {
	newAccessToken, newRefreshToken, err := utilities.RefreshTokens(refreshToken)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	return &TokenResponse{
		Access:  newAccessToken,
		Refresh: newRefreshToken,
	}, nil
}
