package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	MaxImageSize             = 5 * 1024 * 1024
	uploadAuthorizationTTL   = 15 * time.Minute
	downloadAuthorizationTTL = 5 * time.Minute
)

var (
	ErrImageNotOwned          = errors.New("image is not owned by user")
	ErrInvalidImageState      = errors.New("invalid image state")
	ErrInvalidImageMetadata   = errors.New("invalid image metadata")
	ErrImageVerification      = errors.New("image verification failed")
	ErrObjectStoreUnavailable = errors.New("object store is unavailable")
)

type ImageService struct {
	db          *gorm.DB
	objectStore ObjectStore
	verifier    utils.ImageVerifier
}

func NewImageService(
	db *gorm.DB,
	objectStore ObjectStore,
	verifier utils.ImageVerifier,
) *ImageService {
	return &ImageService{db: db, objectStore: objectStore, verifier: verifier}
}

type BeginImageUploadRequest struct {
	ListingID        *uuid.UUID
	ExpectedSize     int64
	ExpectedMimeType string
	Position         int
}

type BeginImageUploadResult struct {
	Image         models.Image
	Authorization UploadAuthorization
}

func (s *ImageService) BeginUpload(
	ctx context.Context,
	actorID uuid.UUID,
	request BeginImageUploadRequest,
) (*BeginImageUploadResult, error) {
	if request.ExpectedSize <= 0 || request.ExpectedSize > MaxImageSize {
		return nil, ErrInvalidImageMetadata
	}
	if !isAllowedImageMimeType(request.ExpectedMimeType) || request.Position < 0 {
		return nil, ErrInvalidImageMetadata
	}
	imageID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate image ID: %w", err)
	}
	now := time.Now().UTC()
	image := models.Image{
		ID:                imageID,
		UploadedByID:      actorID,
		ListingID:         request.ListingID,
		Status:            models.ImageStatusPending,
		ObjectKey:         createObjectKey(actorID, request.ListingID, imageID),
		Position:          request.Position,
		ExpectedSizeBytes: request.ExpectedSize,
		ExpectedMimeType:  request.ExpectedMimeType,
		UploadExpiresAt:   now.Add(uploadAuthorizationTTL),
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if request.ListingID != nil {
			if err := s.authorizeDraftListing(
				ctx,
				tx,
				actorID,
				*request.ListingID,
			); err != nil {
				return err
			}
		}
		return gorm.G[models.Image](tx).Create(ctx, &image)
	})
	if err != nil {
		return nil, err
	}

	authorization, err := s.objectStore.AuthorizeUpload(
		ctx,
		UploadAuthorizationRequest{
			ObjectKey:    image.ObjectKey,
			MimeType:     image.ExpectedMimeType,
			MaximumBytes: MaxImageSize,
			ExpiresAt:    image.UploadExpiresAt,
		},
	)
	if err != nil {
		cleanupErr := s.db.WithContext(ctx).
			Unscoped().
			Delete(&models.Image{}, "id = ?", image.ID).
			Error
		return nil, errors.Join(err, cleanupErr)
	}

	return &BeginImageUploadResult{
		Image:         image,
		Authorization: authorization,
	}, nil
}

func (s *ImageService) CompleteUpload(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) (*models.Image, error) {
	image, err := s.getImageByID(ctx, s.db, imageID)
	if err != nil {
		return nil, err
	}
	if image.UploadedByID != actorID {
		return nil, ErrImageNotOwned
	}
	if image.Status == models.ImageStatusReady {
		return &image, nil
	}
	if image.Status != models.ImageStatusPending {
		return nil, ErrInvalidImageState
	}

	rows, err := gorm.G[models.Image](s.db).
		Where("id = ? AND status = ?", imageID, models.ImageStatusPending).
		Update(ctx, "status", models.ImageStatusVerifying)
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, ErrInvalidImageState
	}

	verified, err := s.verifyObject(ctx, image)
	if err != nil {
		var transitionErr error
		if errors.Is(err, ErrInvalidImageMetadata) ||
			errors.Is(err, ErrImageVerification) {
			transitionErr = s.markFailed(ctx, image)
		} else {
			transitionErr = s.resetPending(ctx, imageID)
		}
		return nil, errors.Join(err, transitionErr)
	}

	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":          models.ImageStatusReady,
			"size_bytes":      verified.SizeBytes,
			"mime_type":       verified.MimeType,
			"width":           verified.Width,
			"height":          verified.Height,
			"checksum_sha256": verified.ChecksumSHA256,
			"verified_at":     now,
		}
		rows := tx.Model(&models.Image{}).
			Where("id = ? AND status = ?", imageID, models.ImageStatusVerifying).
			Updates(updates).RowsAffected
		if rows == 0 {
			return ErrInvalidImageState
		}
		if image.ListingID == nil {
			rows, err := gorm.G[models.User](tx).
				Where("id = ?", actorID).
				Update(ctx, "profile_image_id", imageID)
			if err != nil {
				return err
			}
			if rows == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(err, s.resetPending(ctx, imageID))
	}

	ready, err := s.getImageByID(ctx, s.db, imageID)
	return &ready, err
}

func (s *ImageService) getImageByID(
	ctx context.Context,
	db *gorm.DB,
	imageID uuid.UUID,
) (models.Image, error) {
	return gorm.G[models.Image](db).Where("id = ?", imageID).First(ctx)
}

func (s *ImageService) GetMetadata(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) (*models.Image, error) {
	image, err := s.getImageByID(ctx, s.db, imageID)
	if err != nil {
		return nil, err
	}
	if image.UploadedByID != actorID {
		return nil, ErrImageNotOwned
	}
	return &image, nil
}

func (s *ImageService) GetDownloadURL(
	ctx context.Context,
	imageID uuid.UUID,
) (string, error) {
	image, err := s.getImageByID(ctx, s.db, imageID)
	if err != nil {
		return "", err
	}
	if image.Status != models.ImageStatusReady {
		return "", gorm.ErrRecordNotFound
	}
	expiresAt := time.Now().UTC().Add(downloadAuthorizationTTL)
	return s.objectStore.AuthorizeDownload(ctx, image.ObjectKey, expiresAt)
}

func (s *ImageService) Remove(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		image, err := s.getImageByID(ctx, tx, imageID)
		if err != nil {
			return err
		}
		if image.UploadedByID != actorID {
			return ErrImageNotOwned
		}
		if image.Status == models.ImageStatusDeleting {
			return nil
		}
		if image.Status == models.ImageStatusVerifying {
			return ErrInvalidImageState
		}

		updates := map[string]any{}
		if image.ListingID != nil && image.DetachedAt == nil {
			updates["detached_at"] = time.Now().UTC()
		}

		if _, err := gorm.G[models.User](tx).
			Where("id = ? AND profile_image_id = ?", actorID, imageID).
			Update(ctx, "profile_image_id", nil); err != nil {
			return err
		}

		hasOrderReference, err := s.hasOrderReference(ctx, tx, imageID)
		if err != nil {
			return err
		}
		if !hasOrderReference {
			updates["status"] = models.ImageStatusDeleting
		}
		if len(updates) == 0 {
			return nil
		}

		result := tx.Model(&models.Image{}).
			Where("id = ?", imageID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *ImageService) verifyObject(
	ctx context.Context,
	image models.Image,
) (utils.VerifiedImage, error) {
	metadata, err := s.objectStore.Stat(ctx, image.ObjectKey)
	if err != nil {
		return utils.VerifiedImage{}, err
	}
	if metadata.SizeBytes != image.ExpectedSizeBytes ||
		metadata.SizeBytes > MaxImageSize {
		return utils.VerifiedImage{}, ErrInvalidImageMetadata
	}

	reader, err := s.objectStore.Open(ctx, image.ObjectKey)
	if err != nil {
		return utils.VerifiedImage{}, err
	}
	defer reader.Close()

	verified, err := s.verifier.Verify(ctx, reader, utils.VerificationOptions{
		MaximumSizeBytes:  MaxImageSize,
		ExpectedSizeBytes: image.ExpectedSizeBytes,
		ExpectedMimeType:  image.ExpectedMimeType,
	})
	if err != nil {
		return utils.VerifiedImage{}, fmt.Errorf("%w: %v", ErrImageVerification, err)
	}
	return verified, nil
}

func (s *ImageService) authorizeDraftListing(
	ctx context.Context,
	db *gorm.DB,
	actorID uuid.UUID,
	listingID uuid.UUID,
) error {
	listing, err := gorm.G[models.Listing](db).
		Where("id = ?", listingID).
		First(ctx)
	if err != nil {
		return err
	}
	if listing.SellerID != actorID {
		return ErrImageNotOwned
	}
	if listing.Status != models.ListingStatusDraft {
		return ErrInvalidImageState
	}
	return nil
}

func (s *ImageService) hasOrderReference(
	ctx context.Context,
	db *gorm.DB,
	imageID uuid.UUID,
) (bool, error) {
	count, err := gorm.G[models.Order](db).
		Where("first_image_id = ?", imageID).
		Count(ctx, "id")
	return count > 0, err
}

func (s *ImageService) markFailed(ctx context.Context, image models.Image) error {
	updates := map[string]any{"status": models.ImageStatusFailed}
	if image.ListingID != nil {
		updates["detached_at"] = time.Now().UTC()
	}
	result := s.db.WithContext(ctx).
		Model(&models.Image{}).
		Where("id = ? AND status = ?", image.ID, models.ImageStatusVerifying).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInvalidImageState
	}
	return nil
}

func (s *ImageService) resetPending(ctx context.Context, imageID uuid.UUID) error {
	rows, err := gorm.G[models.Image](s.db).
		Where("id = ? AND status = ?", imageID, models.ImageStatusVerifying).
		Update(ctx, "status", models.ImageStatusPending)
	if err == nil && rows == 0 {
		return ErrInvalidImageState
	}
	return err
}

func createObjectKey(
	actorID uuid.UUID,
	listingID *uuid.UUID,
	imageID uuid.UUID,
) string {
	if listingID == nil {
		return fmt.Sprintf("users/%s/%s", actorID, imageID)
	}
	return fmt.Sprintf("listings/%s/%s", *listingID, imageID)
}

func isAllowedImageMimeType(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/png"
}
