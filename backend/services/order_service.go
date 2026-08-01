package services

import (
	"context"
	"errors"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCannotPurchaseOwnListing = errors.New("cannot purchase own listing")

type OrderService struct {
	db *gorm.DB
}

func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

// CreateFromListing creates an order from a Listing model (server-side data)
func (s *OrderService) CreateFromListing(
	ctx context.Context,
	buyerID uuid.UUID,
	listing *models.Listing,
) (*models.Order, error) {
	return s.Create(ctx, buyerID, listing.ID)
}

func (s *OrderService) Create(
	ctx context.Context,
	buyerID uuid.UUID,
	listingID uuid.UUID,
) (*models.Order, error) {
	var createdOrder *models.Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var listing models.Listing
		err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", listingID).
			First(&listing).
			Error
		if err != nil {
			return err
		}
		if listing.Status != models.ListingStatusAvailable {
			return gorm.ErrRecordNotFound
		}
		if listing.SellerID == buyerID {
			return ErrCannotPurchaseOwnListing
		}
		seller, err := gorm.G[models.User](tx).
			Where("id = ?", listing.SellerID).
			First(ctx)
		if err != nil {
			return err
		}
		images, err := gorm.G[models.Image](tx).
			Where("listing_id = ? AND status = ?", listing.ID, models.ImageStatusReady).
			Order("position ASC, id ASC").
			Find(ctx)
		if err != nil {
			return err
		}

		order := models.Order{
			BuyerID:      buyerID,
			ListingID:    listing.ID,
			Title:        listing.Title,
			Description:  listing.Description,
			Price:        listing.Price,
			FirstImageID: nil,
			SellerName:   seller.FirstName + " " + seller.LastName,
			Status:       "Completed",
			PurchasedAt:  time.Now().UTC(),
		}

		// Set first image ID if available
		if len(images) > 0 {
			order.FirstImageID = &images[0].ID
		}

		if err := gorm.G[models.Order](tx).Create(ctx, &order); err != nil {
			return err
		}

		rowsAffected, err := gorm.G[models.Listing](tx).
			Where("id = ? AND status = ?", listing.ID, models.ListingStatusAvailable).
			Update(ctx, "status", models.ListingStatusSold)
		if err != nil {
			return err
		}

		if rowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		createdOrder = &order
		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdOrder, nil
}

func (s *OrderService) GetByBuyerID(
	ctx context.Context,
	buyerID uuid.UUID,
) ([]models.Order, error) {
	return gorm.G[models.Order](s.db).
		Where("buyer_id = ?", buyerID).
		Order("purchased_at DESC").
		Find(ctx)
}
