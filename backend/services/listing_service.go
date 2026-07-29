package services

import (
	"context"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ListingService struct {
	db *gorm.DB
}

func NewListingService(db *gorm.DB) *ListingService {
	return &ListingService{db: db}
}

func ActiveImagesOnly(db gorm.PreloadBuilder) error {
	db.Where("status = ?", models.ImageStatusReady).
		Order("position asc, id asc")
	return nil
}

// User CURSOR to track last returned listing by ID
func (s *ListingService) Search(
	ctx context.Context,
	query string,
	limit int,
	cursor uuid.UUID,
) ([]models.Listing, error) {

	queryObj := gorm.G[models.Listing](s.db).
		Preload("Seller", nil).
		Preload("Images", ActiveImagesOnly).
		Where("title LIKE ?", "%"+query+"%").
		Where("status = ?", models.ListingStatusAvailable).
		Order("id DESC").
		Limit(limit)

	if cursor != uuid.Nil {
		queryObj = queryObj.Where("id < ?", cursor)
	}

	return queryObj.Find(ctx)
}

// User CURSOR to track last returned listing by ID
func (s *ListingService) GetAll(
	ctx context.Context,
	limit int,
	cursor uuid.UUID,
) ([]models.Listing, error) {

	queryObj := gorm.G[models.Listing](s.db).
		Preload("Seller", nil).
		Preload("Images", ActiveImagesOnly).
		Where("status = ?", models.ListingStatusAvailable).
		Order("id DESC").
		Limit(limit)

	if cursor != uuid.Nil {
		queryObj = queryObj.Where("id < ?", cursor)
	}

	return queryObj.Find(ctx)
}

func (s *ListingService) GetBySellerID(
	ctx context.Context,
	sellerID uuid.UUID,
) ([]models.Listing, error) {
	return gorm.G[models.Listing](s.db).
		Preload("Seller", nil).
		Preload("Images", ActiveImagesOnly).
		Where("seller_id = ?", sellerID).
		Order("id DESC").
		Find(ctx)
}

func (s *ListingService) GetByID(ctx context.Context, id uuid.UUID) (models.Listing, error) {
	return gorm.G[models.Listing](s.db).
		Preload("Seller", nil).
		Preload("Images", ActiveImagesOnly).
		Where("id = ?", id).
		First(ctx)
}

type CreateListingRequest struct {
	Title       string
	Description string
	Price       float64
	SellerID    uuid.UUID
}

func (s *ListingService) Create(
	ctx context.Context,
	request CreateListingRequest,
) (*models.Listing, error) {
	listing := models.Listing{
		Title:       request.Title,
		Description: request.Description,
		Price:       request.Price,
		SellerID:    request.SellerID,
		Status:      models.ListingStatusDraft,
	}
	if err := gorm.G[models.Listing](s.db).Create(ctx, &listing); err != nil {
		return nil, err
	}
	return &listing, nil
}

type UpdateListingRequest struct {
	Title       string
	Description string
	Price       float64
}

func (s *ListingService) Update(
	ctx context.Context,
	id uuid.UUID,
	req UpdateListingRequest,
) (*models.Listing, error) {
	rows, err := gorm.G[models.Listing](s.db).
		Where("id = ?", id).
		Select("Title", "Description", "Price").
		Updates(ctx, models.Listing{
			Title:       req.Title,
			Description: req.Description,
			Price:       req.Price,
		})
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	// Get updated listing
	listing, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &listing, nil
}

func (s *ListingService) Publish(
	ctx context.Context,
	id uuid.UUID,
	sellerID uuid.UUID,
) (*models.Listing, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var listing models.Listing
		err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND seller_id = ?", id, sellerID).
			First(&listing).
			Error
		if err != nil {
			return err
		}
		if listing.Status == models.ListingStatusAvailable {
			return nil
		}
		if listing.Status != models.ListingStatusDraft {
			return ErrInvalidImageState
		}

		unresolvedImages, err := gorm.G[models.Image](tx).
			Where("listing_id = ?", id).
			Where("status IN ?", []models.ImageStatus{
				models.ImageStatusPending,
				models.ImageStatusVerifying,
			}).
			Count(ctx, "id")
		if err != nil {
			return err
		}
		if unresolvedImages > 0 {
			return ErrInvalidImageState
		}

		rows, err := gorm.G[models.Listing](tx).
			Where(
				"id = ? AND seller_id = ? AND status = ?",
				id,
				sellerID,
				models.ListingStatusDraft,
			).
			Update(ctx, "status", models.ListingStatusAvailable)
		if err != nil {
			return err
		}
		if rows == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	listing, err := s.GetByID(ctx, id)
	return &listing, err
}

func (s *ListingService) AbortDraft(
	ctx context.Context,
	id uuid.UUID,
	sellerID uuid.UUID,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		listing, err := s.lockOwnedListing(ctx, tx, id, sellerID)
		if err != nil {
			return err
		}
		if listing.Status != models.ListingStatusDraft {
			return ErrInvalidImageState
		}
		if err := s.markUnretainedImagesDeleting(ctx, tx, id); err != nil {
			return err
		}
		return deleteListingRow(ctx, tx, id)
	})
}

func (s *ListingService) Delete(
	ctx context.Context,
	id uuid.UUID,
	sellerID uuid.UUID,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.lockOwnedListing(ctx, tx, id, sellerID); err != nil {
			return err
		}
		if err := s.markUnretainedImagesDeleting(ctx, tx, id); err != nil {
			return err
		}
		return deleteListingRow(ctx, tx, id)
	})
}

func (s *ListingService) lockOwnedListing(
	ctx context.Context,
	tx *gorm.DB,
	id uuid.UUID,
	sellerID uuid.UUID,
) (models.Listing, error) {
	var listing models.Listing
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND seller_id = ?", id, sellerID).
		First(&listing).
		Error
	return listing, err
}

func (s *ListingService) markUnretainedImagesDeleting(
	ctx context.Context,
	tx *gorm.DB,
	listingID uuid.UUID,
) error {
	images, err := gorm.G[models.Image](tx).
		Where("listing_id = ?", listingID).
		Find(ctx)
	if err != nil {
		return err
	}
	for _, image := range images {
		orderReferences, err := gorm.G[models.Order](tx).
			Where("first_image_id = ?", image.ID).
			Count(ctx, "id")
		if err != nil {
			return err
		}
		if orderReferences > 0 {
			continue
		}
		if _, err := gorm.G[models.Image](tx).
			Where("id = ?", image.ID).
			Update(ctx, "status", models.ImageStatusDeleting); err != nil {
			return err
		}
	}
	return nil
}

func deleteListingRow(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	rows, err := gorm.G[models.Listing](tx).Where("id = ?", id).Delete(ctx)
	if err != nil {
		return err
	}
	if rows == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
