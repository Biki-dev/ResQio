package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenExpired = errors.New("token has expired")
)

// AccountType distinguishes between standard users and providers
type AccountType string

const (
	AccountTypeUser     AccountType = "user"
	AccountTypeProvider AccountType = "provider"
)

// AuthClaims represents the payload in our JWT
type AuthClaims struct {
	AccountID   string      `json:"account_id"`
	AccountType AccountType `json:"account_type"`
	Role        string      `json:"role"`
	Phone       string      `json:"phone,omitempty"`
	Email       string      `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// HashPassword hashes a plaintext password with bcrypt default cost
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPasswordHash compares a plaintext password with a bcrypt hash
func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken creates a signed JWT string for an authenticated user or provider
func GenerateToken(accountID string, accountType AccountType, role, phone, email, secret string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := AuthClaims{
		AccountID:   accountID,
		AccountType: accountType,
		Role:        role,
		Phone:       phone,
		Email:       email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   accountID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

// ValidateToken verifies and parses the JWT token string
func ValidateToken(tokenString, secret string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AuthClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
