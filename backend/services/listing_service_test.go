package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var testListing models.Listing

// ── Create ───────────────────────────────────────────────────────────────────

func TestCreateListing(t *testing.T) {
	ctx := context.Background()
	service := services.NewListingService(db)

	listing, err := service.Create(ctx, services.CreateListingRequest{
		Title:       "Test Textbook",
		Description: "A test listing",
		Price:       9.99,
		SellerID:    testUser.ID,
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if listing.ID == uuid.Nil {
		t.Error("Expected listing to have an ID after creation")
	}

	listing, err = service.Publish(ctx, listing.ID, testUser.ID)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	testListing = *listing
}

// ── GetByID ──────────────────────────────────────────────────────────────────

func TestGetListingByID_Found(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	result, err := svc.GetByID(ctx, testListing.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.ID != testListing.ID {
		t.Errorf("Expected listing ID %v, got %v", testListing.ID, result.ID)
	}
	if result.Title != testListing.Title {
		t.Errorf("Expected title %v, got %v", testListing.Title, result.Title)
	}
}

func TestGetListingByID_NotFound(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	_, err := svc.GetByID(ctx, uuid.New())
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestGetListingByID_InvalidID(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	_, err := svc.GetByID(ctx, uuid.Nil)
	if err == nil {
		t.Error("Expected error for invalid ID, got nil")
	}
}

// ── GetAll ───────────────────────────────────────────────────────────────────

func TestGetAll_ReturnsResults(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	results, err := svc.GetAll(ctx, 10, uuid.Nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected at least one listing, got none")
	}
}

func TestGetAll_LimitIsRespected(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	// Create extra listings to ensure limit is meaningful
	for range 3 {
		_, _ = svc.Create(ctx, services.CreateListingRequest{
			Title:    "Extra Listing",
			Price:    1.00,
			SellerID: testUser.ID,
		})
	}

	results, err := svc.GetAll(ctx, 2, uuid.Nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Expected at most 2 results, got %d", len(results))
	}
}

func TestGetAll_CursorPagination(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	firstPage, err := svc.GetAll(ctx, 1, uuid.Nil)
	if err != nil {
		t.Fatalf("Expected no error on first page, got %v", err)
	}
	if len(firstPage) == 0 {
		t.Fatal("Expected at least one result on first page")
	}

	cursor := firstPage[0].ID
	secondPage, err := svc.GetAll(ctx, 10, cursor)
	if err != nil {
		t.Fatalf("Expected no error on second page, got %v", err)
	}
	for _, l := range secondPage {
		if l.ID.String() >= cursor.String() {
			t.Errorf("Expected all results to have ID less than cursor %v, got %v", cursor, l.ID)
		}
	}
}

// ── GetBySellerID ─────────────────────────────────────────────────────────────

func TestGetBySellerID_Found(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	results, err := svc.GetBySellerID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected at least one listing for test seller")
	}
	for _, l := range results {
		if l.SellerID != testUser.ID {
			t.Errorf("Expected seller ID %v, got %v", testUser.ID, l.SellerID)
		}
	}
}

func TestGetBySellerID_NoResults(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	results, err := svc.GetBySellerID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected no listings, got %d", len(results))
	}
}

// ── Search ───────────────────────────────────────────────────────────────────

func TestSearch_MatchingQuery(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	results, err := svc.Search(ctx, "Textbook", 10, uuid.Nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected at least one result for matching query")
	}
}

func TestSearch_NoMatch(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	results, err := svc.Search(ctx, "zzznomatchzzz", 10, uuid.Nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected no results, got %d", len(results))
	}
}

func TestSearch_MatchesTitleOnly(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)
	_, err := svc.Create(ctx, services.CreateListingRequest{
		Title:       "Ordinary title",
		Description: "description-only-search-term",
		Price:       1,
		SellerID:    testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	results, err := svc.Search(ctx, "description-only-search-term", 10, uuid.Nil)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() returned %d description matches, want 0", len(results))
	}
}

func TestPublishListingRejectsUnresolvedImages(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)
	listing, err := svc.Create(ctx, services.CreateListingRequest{
		Title: "Pending image", Description: "Description", Price: 1, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	image := models.Image{
		UploadedByID:      testUser.ID,
		ListingID:         &listing.ID,
		Status:            models.ImageStatusPending,
		ObjectKey:         "test/pending/" + listing.ID.String(),
		ExpectedSizeBytes: 1,
		ExpectedMimeType:  "image/png",
		UploadExpiresAt:   time.Now().Add(time.Minute),
	}
	if err := gorm.G[models.Image](db).Create(ctx, &image); err != nil {
		t.Fatalf("create pending image: %v", err)
	}

	if _, err := svc.Publish(ctx, listing.ID, testUser.ID); !errors.Is(
		err,
		services.ErrInvalidImageState,
	) {
		t.Fatalf("Publish() error = %v, want invalid image state", err)
	}
}

func TestGetListingPreloadsOnlyReadyAttachedImages(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)
	listing, err := svc.Create(ctx, services.CreateListingRequest{
		Title: "Filtered images", Description: "Description", Price: 1, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	images := []models.Image{
		testListingImage(listing.ID, "ready", models.ImageStatusReady, nil),
		testListingImage(listing.ID, "pending", models.ImageStatusPending, nil),
	}
	if err := gorm.G[models.Image](db).CreateInBatches(ctx, &images, len(images)); err != nil {
		t.Fatalf("create images: %v", err)
	}

	result, err := svc.GetByID(ctx, listing.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].ObjectKey != "test/ready" {
		t.Errorf("Images = %#v, want only ready attached image", result.Images)
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

// func TestUpdateListing(t *testing.T) {
// 	ctx := context.Background()
// 	svc := services.NewListingService(db)
//
// 	_, err := svc.Update(ctx, &testListing, map[string]any{
// 		"title": "Updated Title",
// 		"price": 19.99,
// 	})
// 	if err != nil {
// 		t.Fatalf("Expected no error, got %v", err)
// 	}
//
// 	result, err := svc.GetByID(ctx, testListing.ID)
// 	if err != nil {
// 		t.Fatalf("Expected no error fetching updated listing, got %v", err)
// 	}
// 	if result.Title != "Updated Title" {
// 		t.Errorf("Expected title 'Updated Title', got %v", result.Title)
// 	}
// 	if result.Price != 19.99 {
// 		t.Errorf("Expected price 19.99, got %v", result.Price)
// 	}
// }

// ── Delete ───────────────────────────────────────────────────────────────────

func TestDeleteListing(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	listing, err := svc.Create(ctx, services.CreateListingRequest{
		Title:    "To Be Deleted",
		Price:    5.00,
		SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	err = svc.Delete(ctx, listing.ID, testUser.ID)
	if err != nil {
		t.Fatalf("Expected no error on delete, got %v", err)
	}

	_, err = svc.GetByID(ctx, listing.ID)
	if err == nil {
		t.Error("Expected error fetching deleted listing, got nil")
	}
}

func TestDeleteListing_InvalidID(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	err := svc.Delete(ctx, uuid.Nil, testUser.ID)
	if err == nil {
		t.Error("Expected error for invalid UUID, got nil")
	}
}

func TestDeleteListing_NotFound(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)

	err := svc.Delete(ctx, uuid.Nil, testUser.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Expected error for missing record")
	}
}

func TestAbortDraftMarksImagesDeleting(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)
	listing, err := svc.Create(ctx, services.CreateListingRequest{
		Title: "Aborted draft", Description: "Description",
		Price: 1, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	images := []models.Image{
		testListingImage(listing.ID, "abort-ready", models.ImageStatusReady, nil),
		testListingImage(listing.ID, "abort-pending", models.ImageStatusPending, nil),
	}
	if err := gorm.G[models.Image](db).CreateInBatches(ctx, &images, len(images)); err != nil {
		t.Fatalf("create images: %v", err)
	}

	if err := svc.AbortDraft(ctx, listing.ID, testUser.ID); err != nil {
		t.Fatalf("AbortDraft() error = %v", err)
	}
	for _, image := range images {
		stored, err := gorm.G[models.Image](db).
			Where("id = ?", image.ID).
			First(ctx)
		if err != nil {
			t.Fatalf("load image: %v", err)
		}
		if stored.Status != models.ImageStatusDeleting {
			t.Errorf("image status = %q, want deleting", stored.Status)
		}
	}
}

func TestAbortDraftRejectsPublishedListing(t *testing.T) {
	ctx := context.Background()
	svc := services.NewListingService(db)
	listing, err := svc.Create(ctx, services.CreateListingRequest{
		Title: "Published listing", Description: "Description",
		Price: 1, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := svc.Publish(ctx, listing.ID, testUser.ID); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := svc.AbortDraft(
		ctx,
		listing.ID,
		testUser.ID,
	); !errors.Is(err, services.ErrInvalidImageState) {
		t.Fatalf("AbortDraft() error = %v, want invalid state", err)
	}
}

func testListingImage(
	listingID uuid.UUID,
	name string,
	status models.ImageStatus,
	_ *time.Time,
) models.Image {
	return models.Image{
		UploadedByID:      testUser.ID,
		ListingID:         &listingID,
		Status:            status,
		ObjectKey:         "test/" + name,
		StagingObjectKey:  "staging/test/" + name,
		ExpectedSizeBytes: 1,
		ExpectedMimeType:  "image/png",
		UploadExpiresAt:   time.Now().Add(time.Minute),
	}
}
