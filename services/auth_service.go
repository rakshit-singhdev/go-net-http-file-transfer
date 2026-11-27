package services

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"file-sharing-server/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidInput = errors.New("invalid registration input")
)

type UserRepository interface {
	Create(ctx context.Context, user models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
}

type AuthService struct {
	userRepository UserRepository
	jwtSecret      []byte
	tokenDuration  time.Duration
}

func NewAuthService(
	userRepository UserRepository,
	jwtSecret string,
	tokenDuration time.Duration,
) *AuthService {
	return &AuthService{
		userRepository: userRepository,
		jwtSecret:      []byte(jwtSecret),
		tokenDuration:  tokenDuration,
	}
}

// Register creates a new user account.
func (s *AuthService) Register(
	ctx context.Context,
	name string,
	email string,
	password string,
) (*models.User, error) {

	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" || email == "" || len(password) < 8 {
		return nil, ErrInvalidInput
	}

	existingUser, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if existingUser != nil {
		return nil, ErrEmailAlreadyExists
	}

	passwordHash := hashPassword(password)

	now := time.Now().UTC()

	user := models.User{
		ID:           bson.NewObjectID(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         models.RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepository.Create(ctx, user); err != nil {
		return nil, err
	}

	return &user, nil
}

// Login verifies credentials and generates an access token.
func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (string, *models.User, error) {

	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}

	if user == nil || !verifyPassword(password, user.PasswordHash) {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// generateToken creates a signed JWT.
func (s *AuthService) generateToken(user *models.User) (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   user.ID.Hex(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenDuration)),
		Issuer:    "file-sharing-server",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(s.jwtSecret)
}

// hashPassword generates an Argon2id password hash.
func hashPassword(password string) string {
	const (
		memory      = 64 * 1024
		iterations  = 3
		parallelism = 2
		saltLength  = 16
		keyLength   = 32
	)

	salt := make([]byte, saltLength)

	// Generate a cryptographically secure random salt.
	// crypto/rand is used in the helper below.
	if _, err := secureRandom(salt); err != nil {
		panic(fmt.Errorf("generate password salt: %w", err))
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$%s$%s",
		encodedSalt,
		encodedHash,
	)
}

// verifyPassword compares a password with its stored Argon2id hash.
func verifyPassword(password string, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 4 || parts[1] != "argon2id" {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		3,
		64*1024,
		2,
		uint32(len(expectedHash)),
	)

	return subtleCompare(actualHash, expectedHash)
}

func secureRandom(destination []byte) (int, error) {
	return rand.Read(destination)
}

func subtleCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}