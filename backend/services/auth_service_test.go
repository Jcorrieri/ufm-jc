package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestAuthBadPassword(t *testing.T) {
	ctx := context.Background()
	authService := services.NewAuthService(db, "test-secret")
	_, _, err := authService.Authenticate(ctx, testUser.Email, "bad_password")

	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("Expected %v, got %v", bcrypt.ErrMismatchedHashAndPassword, err)
	}
}

func TestAuthBadEmail(t *testing.T) {
	ctx := context.Background()
	authService := services.NewAuthService(db, "test-secret")
	_, _, err := authService.Authenticate(ctx, "bad@ufl.edu", testUser.PasswordHash)

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Expected %v, got %v", gorm.ErrRecordNotFound, err)
	}
}

func TestAuthenticateUsesInjectedJWTSecret(t *testing.T) {
	ctx := context.Background()
	authService := services.NewAuthService(db, "injected-secret")

	user, token, err := authService.Authenticate(ctx, testUser.Email, "password")
	if err != nil {
		t.Fatalf("Authenticate() returned an error: %v", err)
	}

	claims, err := utils.ValidateToken(token, "injected-secret")
	if err != nil {
		t.Fatalf("ValidateToken() returned an error: %v", err)
	}
	if claims.Subject != user.ID.String() {
		t.Errorf("token subject = %q, want %q", claims.Subject, user.ID)
	}
}
