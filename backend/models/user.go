package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	// Using UUID v7; See https://uuid7.com
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Use a partial index so soft-deleted email addresses can be reused.
	// See https://sqlite.org/partialindex.html.
	Email          string     `gorm:"uniqueIndex:idx_email_active,where:deleted_at IS NULL;not null"`
	PasswordHash   string     `json:"-" gorm:"not null"`
	FirstName      string     `gorm:"not null"`
	LastName       string     `gorm:"not null"`
	ProfileImageID *uuid.UUID `gorm:"type:uuid;index"`
	ProfileImage   *Image     `json:"-" gorm:"foreignKey:ProfileImageID"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// NOTE: https://gorm.io/docs/hooks.html
func (u *User) BeforeCreate(tx *gorm.DB) error {
	id, err := uuid.NewV7()
	u.ID = id
	return err
}

// The actual JSON object returned by the API
type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	ImageID   *uuid.UUID `json:"image_id"`
	CreatedAt time.Time  `json:"created_at"`
}

func (u *User) GetResponse() UserResponse {
	var imageID *uuid.UUID
	if u.ProfileImageID != nil {
		imageID = u.ProfileImageID
	} else if u.ProfileImage != nil && u.ProfileImage.ID != uuid.Nil {
		imageID = &u.ProfileImage.ID
	}
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		ImageID:   imageID,
		CreatedAt: u.CreatedAt,
	}
}
