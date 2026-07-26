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
	ErrImageStillReferenced   = errors.New("image is still referenced")
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
		s.hardDeleteMetadata(ctx, image.ID)
		return nil, err
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
	image, err := s.GetImageByID(ctx, imageID)
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
		if isDeterministicVerificationError(err) {
			s.markFailed(ctx, imageID)
		} else {
			s.resetPending(ctx, imageID)
		}
		return nil, err
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
			result := tx.Model(&models.User{}).
				Where("id = ?", actorID).
				Update("profile_image_id", imageID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
	if err != nil {
		s.resetPending(ctx, imageID)
		return nil, err
	}

	ready, err := s.GetImageByID(ctx, imageID)
	return &ready, err
}

func (s *ImageService) GetImageByID(
	ctx context.Context,
	imageID uuid.UUID,
) (models.Image, error) {
	return gorm.G[models.Image](s.db).Where("id = ?", imageID).First(ctx)
}

func (s *ImageService) GetMetadata(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) (*models.Image, error) {
	image, err := s.GetImageByID(ctx, imageID)
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
	image, err := s.GetImageByID(ctx, imageID)
	if err != nil {
		return "", err
	}
	if image.Status != models.ImageStatusReady {
		return "", gorm.ErrRecordNotFound
	}
	expiresAt := time.Now().UTC().Add(downloadAuthorizationTTL)
	return s.objectStore.AuthorizeDownload(ctx, image.ObjectKey, expiresAt)
}

func (s *ImageService) Detach(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) error {
	image, err := s.GetMetadata(ctx, actorID, imageID)
	if err != nil {
		return err
	}
	if image.ListingID == nil || image.DetachedAt != nil {
		return ErrInvalidImageState
	}
	now := time.Now().UTC()
	_, err = gorm.G[models.Image](s.db).
		Where("id = ?", imageID).
		Update(ctx, "detached_at", now)
	return err
}

func (s *ImageService) MarkForDeletion(
	ctx context.Context,
	actorID uuid.UUID,
	imageID uuid.UUID,
) error {
	image, err := s.GetMetadata(ctx, actorID, imageID)
	if err != nil {
		return err
	}
	if image.Status == models.ImageStatusDeleting {
		return nil
	}
	if image.Status == models.ImageStatusVerifying {
		return ErrInvalidImageState
	}
	if image.ListingID != nil && image.DetachedAt == nil {
		return ErrImageStillReferenced
	}

	referenced, err := s.isReferenced(ctx, imageID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrImageStillReferenced
	}

	_, err = gorm.G[models.Image](s.db).
		Where("id = ?", imageID).
		Update(ctx, "status", models.ImageStatusDeleting)
	return err
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

func (s *ImageService) isReferenced(ctx context.Context, imageID uuid.UUID) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("profile_image_id = ?", imageID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := s.db.WithContext(ctx).Model(&models.Order{}).
		Where("first_image_id = ?", imageID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ImageService) markFailed(ctx context.Context, imageID uuid.UUID) {
	_, _ = gorm.G[models.Image](s.db).
		Where("id = ?", imageID).
		Update(ctx, "status", models.ImageStatusFailed)
}

func (s *ImageService) resetPending(ctx context.Context, imageID uuid.UUID) {
	_, _ = gorm.G[models.Image](s.db).
		Where("id = ? AND status = ?", imageID, models.ImageStatusVerifying).
		Update(ctx, "status", models.ImageStatusPending)
}

func isDeterministicVerificationError(err error) bool {
	return errors.Is(err, ErrInvalidImageMetadata) ||
		errors.Is(err, ErrImageVerification)
}

func (s *ImageService) hardDeleteMetadata(ctx context.Context, imageID uuid.UUID) {
	_ = s.db.WithContext(ctx).Unscoped().Delete(&models.Image{}, "id = ?", imageID).Error
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
