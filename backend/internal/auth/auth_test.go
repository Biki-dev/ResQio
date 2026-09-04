package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "SecretPass123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == password {
		t.Fatal("hash should not equal raw password")
	}

	if err := CheckPasswordHash(password, hash); err != nil {
		t.Fatalf("expected password verification to succeed, got: %v", err)
	}

	if err := CheckPasswordHash("WrongPassword", hash); err == nil {
		t.Fatal("expected password verification to fail with wrong password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key-12345"
	accountID := "c56a4180-65aa-42ec-a945-5fd21dec0538"
	role := "PUBLIC"
	phone := "+1234567890"

	token, err := GenerateToken(accountID, AccountTypeUser, role, phone, "", secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.AccountID != accountID {
		t.Errorf("expected AccountID %s, got %s", accountID, claims.AccountID)
	}
	if claims.AccountType != AccountTypeUser {
		t.Errorf("expected AccountType %s, got %s", AccountTypeUser, claims.AccountType)
	}
	if claims.Role != role {
		t.Errorf("expected Role %s, got %s", role, claims.Role)
	}
	if claims.Phone != phone {
		t.Errorf("expected Phone %s, got %s", phone, claims.Phone)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	secret := "test-secret-key-12345"
	accountID := "c56a4180-65aa-42ec-a945-5fd21dec0538"

	// Create expired token
	token, err := GenerateToken(accountID, AccountTypeUser, "PUBLIC", "+1234567890", "", secret, -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err == nil {
		t.Fatal("expected expired token validation to return error")
	}
}

func TestValidateTokenWithWrongSecret(t *testing.T) {
	token, err := GenerateToken("test-id", AccountTypeProvider, "ORGANISATION", "", "provider@test.com", "correct-secret", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected validation to fail when secret is mismatched")
	}
}
