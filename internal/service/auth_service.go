package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

// AuthService interface
type AuthService interface {
	Register(user *model.User) error
	Login(username, authhash string) (*model.User, error)
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
	fmt.Println("Received Password:", user.Password) // Debugging

	existingUser, err := s.userRepo.GetUserByEmail(user.Email)
	fmt.Println("Existing User:", existingUser, "Error:", err) // Debugging

	if err == nil && existingUser != nil {
		return errors.New("email already in use")
	}

	if user.Password == "" {
		return errors.New("password cannot be empty")
	}

	// First, apply SHA-256 hashing
	hashedPassword := hash256encode(user.Password) // Store this in DB

	// Store only the SHA-256 hash
	user.Password = hashedPassword

	fmt.Println("Stored SHA-256 Hash:", user.Password) // Debugging

	// Save user to DB
	err = s.userRepo.CreateUser(user)
	if err != nil {
		return errors.New("failed to store user in database")
	}

	return nil
}

// Login function to authenticate user
func (s *authService) Login(username, authhash string) (*model.User, error) {
	// Step 1: Retrieve user from database (already stored as SHA-256 hash)
	user, err := s.userRepo.GetUserByEmail(username)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Step 2: Concatenate email with stored SHA-256 hashed password
	concatenatedString := username + "::" + user.Password // user.Password is already SHA-256 hashed
	fmt.Println("Concatenated String for Bcrypt Check:", concatenatedString)

	// Step 3: Decode Base64 `authhash` (sent by the client)
	bcryptEncryptedBytes, err := base64.StdEncoding.DecodeString(authhash)
	if err != nil {
		return nil, errors.New("invalid authhash format")
	}
	bcryptEncrypted := string(bcryptEncryptedBytes) // Convert bytes to string

	// Step 4: Compare bcrypt hash with concatenated string
	err = bcrypt.CompareHashAndPassword([]byte(bcryptEncrypted), []byte(concatenatedString))
	if err != nil {
		fmt.Println("Bcrypt Comparison Failed:", err)
		return nil, errors.New("invalid credentials")
	}

	// Step 5: Remove password before returning user data
	user.Password = ""

	return user, nil
}
