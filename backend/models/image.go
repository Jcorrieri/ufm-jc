package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImageStatus string

const (
	ImageStatusPending   ImageStatus = "pending"
	ImageStatusVerifying ImageStatus = "verifying"
	ImageStatusReady     ImageStatus = "ready"
	ImageStatusFailed    ImageStatus = "failed"
	ImageStatusDeleting  ImageStatus = "deleting"
)

// Image stores the lifecycle and verified metadata for an object-backed image.
// Image bytes are intentionally kept outside the relational database.
type Image struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UploadedByID uuid.UUID  `gorm:"type:uuid;not null;index"`
	ListingID    *uuid.UUID `gorm:"type:uuid;index"`

	Status           ImageStatus `gorm:"size:32;not null;default:'pending';index"`
	ObjectKey        string      `json:"-" gorm:"not null;uniqueIndex"`
	StagingObjectKey string      `json:"-" gorm:"not null;uniqueIndex"`
	Position         int         `gorm:"not null;default:0"`

	ExpectedSizeBytes int64  `gorm:"not null"`
	ExpectedMimeType  string `gorm:"not null"`

	SizeBytes      *int64
	MimeType       *string
	Width          *int
	Height         *int
	ChecksumSHA256 *string

	UploadExpiresAt       time.Time  `gorm:"not null;index"`
	VerificationStartedAt *time.Time `gorm:"index"`
	VerifiedAt            *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate assigns a UUIDv7 unless the caller preassigned the ID. Upload
// initialization needs to derive the object key from that preassigned ID.
func (image *Image) BeforeCreate(_ *gorm.DB) error {
	if image.ID != uuid.Nil {
		return nil
	}

	id, err := uuid.NewV7()
	image.ID = id
	return err
}

type ImageResponse struct {
	ID                    uuid.UUID   `json:"id"`
	ListingID             *uuid.UUID  `json:"listing_id,omitempty"`
	Status                ImageStatus `json:"status"`
	Position              int         `json:"position"`
	ExpectedSizeBytes     int64       `json:"expected_size"`
	ExpectedMimeType      string      `json:"expected_mime_type"`
	SizeBytes             *int64      `json:"size,omitempty"`
	MimeType              *string     `json:"mime_type,omitempty"`
	Width                 *int        `json:"width,omitempty"`
	Height                *int        `json:"height,omitempty"`
	ChecksumSHA256        *string     `json:"checksum_sha256,omitempty"`
	UploadExpiresAt       time.Time   `json:"upload_expires_at"`
	VerificationStartedAt *time.Time  `json:"verification_started_at,omitempty"`
	VerifiedAt            *time.Time  `json:"verified_at,omitempty"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

func (image *Image) GetResponse() ImageResponse {
	return ImageResponse{
		ID:                    image.ID,
		ListingID:             image.ListingID,
		Status:                image.Status,
		Position:              image.Position,
		ExpectedSizeBytes:     image.ExpectedSizeBytes,
		ExpectedMimeType:      image.ExpectedMimeType,
		SizeBytes:             image.SizeBytes,
		MimeType:              image.MimeType,
		Width:                 image.Width,
		Height:                image.Height,
		ChecksumSHA256:        image.ChecksumSHA256,
		UploadExpiresAt:       image.UploadExpiresAt,
		VerificationStartedAt: image.VerificationStartedAt,
		VerifiedAt:            image.VerifiedAt,
		CreatedAt:             image.CreatedAt,
		UpdatedAt:             image.UpdatedAt,
	}
}
