package services

import (
	"context"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Define the service struct whose dependencies are the db connection and secret.
// Services will handle all database operations for each model (users, posts, etc.).
// See https://gorm.io/docs/the_generics_way.html for generics API usage.
type AuthService struct {
	db        *gorm.DB
	jwtSecret string
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{db: db, jwtSecret: jwtSecret}
}

func (s *AuthService) Authenticate(
	ctx context.Context,
	email string,
	password string,
) (*models.User, string, error) {
	// Check if account exists w/ given email
	user, err := gorm.G[models.User](s.db).Where("email = ?", email).First(ctx)
	if err != nil {
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", err
	}

	// Generate a JWT token for the authenticated user
	token, err := utils.GenerateToken(user.ID, s.jwtSecret)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// Logout performs server-side logout for the supplied session token.
func (s *AuthService) Logout(ctx context.Context, sessionToken string) error {
	// If the application later implements server-side sessions or token
	// blacklisting, revoke the token here (delete DB row / add to denylist).
	// For now, this is a no-op which keeps the service interface stable.
	return nil
}
