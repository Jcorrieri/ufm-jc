package services_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"github.com/google/uuid"
)

type fakeObjectStore struct {
	authorization services.UploadAuthorization
	authorizeErr  error
	metadata      services.ObjectMetadata
	statErr       error
	openErr       error
	promoteErr    error
	promotedFrom  string
	promotedTo    string
	promotedID    string
	downloadURL   string
	uploadRequest services.UploadAuthorizationRequest
}

func (store *fakeObjectStore) AuthorizeUpload(
	_ context.Context,
	request services.UploadAuthorizationRequest,
) (services.UploadAuthorization, error) {
	store.uploadRequest = request
	return store.authorization, store.authorizeErr
}

func (store *fakeObjectStore) Open(
	context.Context,
	string,
) (services.StoredObject, error) {
	if store.statErr != nil {
		return services.StoredObject{}, store.statErr
	}
	if store.openErr != nil {
		return services.StoredObject{}, store.openErr
	}
	metadata := store.metadata
	if metadata.Identity == "" {
		metadata.Identity = "test-etag"
	}
	return services.StoredObject{
		Reader:   io.NopCloser(strings.NewReader("image")),
		Metadata: metadata,
	}, nil
}

func (store *fakeObjectStore) Promote(
	_ context.Context,
	sourceKey string,
	destinationKey string,
	identity string,
) error {
	store.promotedFrom = sourceKey
	store.promotedTo = destinationKey
	store.promotedID = identity
	return store.promoteErr
}

func (store *fakeObjectStore) AuthorizeDownload(
	context.Context,
	string,
	time.Time,
) (string, error) {
	return store.downloadURL, nil
}

func (store *fakeObjectStore) Delete(context.Context, string) error {
	return nil
}

type fakeImageVerifier struct {
	result utils.VerifiedImage
	err    error
	calls  int
}

func (verifier *fakeImageVerifier) Verify(
	context.Context,
	io.Reader,
	utils.VerificationOptions,
) (utils.VerifiedImage, error) {
	verifier.calls++
	return verifier.result, verifier.err
}

func TestImageServiceBeginAndCompleteListingUpload(t *testing.T) {
	ctx := context.Background()
	listingService := services.NewListingService(db)
	listing, err := listingService.Create(ctx, services.CreateListingRequest{
		Title: "Image listing", Description: "Description", Price: 10, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/png"},
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/png", Width: 20, Height: 10,
		ChecksumSHA256: strings.Repeat("a", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)

	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ListingID: listingID(listing.ID), ExpectedSize: 5,
		ExpectedMimeType: "image/png", Position: 1,
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}
	if begin.Image.Status != models.ImageStatusPending {
		t.Fatalf("status = %q, want pending", begin.Image.Status)
	}
	if store.uploadRequest.ObjectKey != begin.Image.StagingObjectKey {
		t.Errorf("authorized key = %q, want staging key", store.uploadRequest.ObjectKey)
	}
	if begin.Image.StagingObjectKey == begin.Image.ObjectKey {
		t.Error("staging and serving keys must differ")
	}

	ready, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if ready.Status != models.ImageStatusReady || ready.ChecksumSHA256 == nil {
		t.Fatalf("completed image = %#v", ready)
	}
	if store.promotedFrom != begin.Image.StagingObjectKey ||
		store.promotedTo != begin.Image.ObjectKey ||
		store.promotedID != "test-etag" {
		t.Errorf(
			"promotion = (%q, %q, %q), want staged identity promotion",
			store.promotedFrom,
			store.promotedTo,
			store.promotedID,
		)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("idempotent CompleteUpload() error = %v", err)
	}
	if verifier.calls != 1 {
		t.Errorf("verifier calls = %d, want 1", verifier.calls)
	}
	store.downloadURL = "https://download.example"
	downloadURL, err := imageService.GetDownloadURL(ctx, begin.Image.ID)
	if err != nil {
		t.Fatalf("GetDownloadURL() error = %v", err)
	}
	if downloadURL != store.downloadURL {
		t.Errorf("download URL = %q, want %q", downloadURL, store.downloadURL)
	}
}

func TestImageServiceCompletesProfileUpload(t *testing.T) {
	ctx := context.Background()
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/jpeg"},
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/jpeg", Width: 10, Height: 10,
		ChecksumSHA256: strings.Repeat("b", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	user, err := services.NewUserService(db).GetByID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if user.ProfileImageID == nil || *user.ProfileImageID != begin.Image.ID {
		t.Errorf("ProfileImageID = %v, want %v", user.ProfileImageID, begin.Image.ID)
	}
}

func TestImageServiceMarksSupersededProfileImageForDeletion(t *testing.T) {
	ctx := context.Background()
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{
			URL: "https://upload.example", Method: "PUT",
		},
		metadata: services.ObjectMetadata{SizeBytes: 5, MimeType: "image/jpeg"},
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/jpeg", Width: 10, Height: 10,
		ChecksumSHA256: strings.Repeat("f", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)

	first, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("first BeginUpload() error = %v", err)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, first.Image.ID); err != nil {
		t.Fatalf("first CompleteUpload() error = %v", err)
	}
	second, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("second BeginUpload() error = %v", err)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, second.Image.ID); err != nil {
		t.Fatalf("second CompleteUpload() error = %v", err)
	}

	oldImage, err := imageService.GetMetadata(ctx, testUser.ID, first.Image.ID)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if oldImage.Status != models.ImageStatusDeleting {
		t.Errorf("old image status = %q, want deleting", oldImage.Status)
	}
}

func TestImageServiceDetachesFailedListingImage(t *testing.T) {
	ctx := context.Background()
	listingService := services.NewListingService(db)
	listing, err := listingService.Create(ctx, services.CreateListingRequest{
		Title: "Failed image listing", Description: "Description",
		Price: 10, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/png"},
	}
	wantError := errors.New("invalid image")
	imageService := services.NewImageService(
		db,
		store,
		&fakeImageVerifier{err: wantError},
	)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ListingID: listingID(listing.ID), ExpectedSize: 5,
		ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}

	if _, err := imageService.CompleteUpload(
		ctx,
		testUser.ID,
		begin.Image.ID,
	); !errors.Is(err, services.ErrImageVerification) {
		t.Fatalf("CompleteUpload() error = %v, want verification error", err)
	}
	image, err := imageService.GetMetadata(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if image.Status != models.ImageStatusFailed {
		t.Errorf("status = %q, want failed", image.Status)
	}
}

func TestImageServiceRejectsInvalidMetadata(t *testing.T) {
	imageService := services.NewImageService(
		db,
		&fakeObjectStore{},
		&fakeImageVerifier{},
	)
	_, err := imageService.BeginUpload(
		context.Background(),
		testUser.ID,
		services.BeginImageUploadRequest{
			ExpectedSize: services.MaxImageSize + 1, ExpectedMimeType: "image/gif",
		},
	)
	if !errors.Is(err, services.ErrInvalidImageMetadata) {
		t.Fatalf("BeginUpload() error = %v, want invalid metadata", err)
	}
}

func TestImageServiceHidesMetadataFromOtherUsers(t *testing.T) {
	ctx := context.Background()
	imageService := services.NewImageService(
		db,
		&fakeObjectStore{
			authorization: services.UploadAuthorization{URL: "https://upload.example"},
		},
		&fakeImageVerifier{},
	)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}

	if _, err := imageService.GetMetadata(
		ctx,
		uuid.New(),
		begin.Image.ID,
	); !errors.Is(err, services.ErrImageNotOwned) {
		t.Fatalf("GetMetadata() error = %v, want not owned", err)
	}
}

func TestImageServiceRemovesMetadataWhenAuthorizationFails(t *testing.T) {
	ctx := context.Background()
	wantError := errors.New("authorization failed")
	store := &fakeObjectStore{authorizeErr: wantError}
	imageService := services.NewImageService(
		db,
		store,
		&fakeImageVerifier{},
	)

	_, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/png",
	})
	if !errors.Is(err, wantError) {
		t.Fatalf("BeginUpload() error = %v, want %v", err, wantError)
	}

	var count int64
	if err := db.Model(&models.Image{}).
		Where("object_key = ?", store.uploadRequest.ObjectKey).
		Count(&count).Error; err != nil {
		t.Fatalf("count image metadata: %v", err)
	}
	if count != 0 {
		t.Errorf("pending image count = %d, want 0", count)
	}
}

func TestImageServiceRetriesTransientStorageFailure(t *testing.T) {
	ctx := context.Background()
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/png"},
		openErr:       services.ErrObjectStoreUnavailable,
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/png", Width: 10, Height: 10,
		ChecksumSHA256: strings.Repeat("c", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}

	if _, err := imageService.CompleteUpload(
		ctx,
		testUser.ID,
		begin.Image.ID,
	); !errors.Is(err, services.ErrObjectStoreUnavailable) {
		t.Fatalf("CompleteUpload() error = %v, want object-store error", err)
	}
	pending, err := imageService.GetMetadata(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if pending.Status != models.ImageStatusPending {
		t.Fatalf("status = %q, want pending", pending.Status)
	}

	store.openErr = nil
	ready, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("retry CompleteUpload() error = %v", err)
	}
	if ready.Status != models.ImageStatusReady {
		t.Errorf("status = %q, want ready", ready.Status)
	}
}

func TestImageServiceRejectsListingImageRemoval(t *testing.T) {
	ctx := context.Background()
	listingService := services.NewListingService(db)
	listing, err := listingService.Create(ctx, services.CreateListingRequest{
		Title: "Remove listing", Description: "Description", Price: 10, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
	}
	imageService := services.NewImageService(db, store, &fakeImageVerifier{})
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ListingID: listingID(listing.ID), ExpectedSize: 5,
		ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}

	if err := imageService.Remove(
		ctx,
		testUser.ID,
		begin.Image.ID,
	); !errors.Is(err, services.ErrInvalidImageState) {
		t.Fatalf("Remove() error = %v, want invalid state", err)
	}
}

func TestImageServiceRemovesProfileImage(t *testing.T) {
	ctx := context.Background()
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/jpeg"},
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/jpeg", Width: 10, Height: 10,
		ChecksumSHA256: strings.Repeat("e", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	if err := imageService.Remove(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	user, err := services.NewUserService(db).GetByID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if user.ProfileImageID != nil {
		t.Errorf("ProfileImageID = %v, want nil", user.ProfileImageID)
	}
	image, err := imageService.GetMetadata(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if image.Status != models.ImageStatusDeleting {
		t.Errorf("status = %q, want deleting", image.Status)
	}
}

func TestImageServiceRejectsRemovalWhileVerifying(t *testing.T) {
	ctx := context.Background()
	imageService := services.NewImageService(
		db,
		&fakeObjectStore{
			authorization: services.UploadAuthorization{URL: "https://upload.example"},
		},
		&fakeImageVerifier{},
	)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ExpectedSize: 5, ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}
	if err := db.Model(&models.Image{}).
		Where("id = ?", begin.Image.ID).
		Update("status", models.ImageStatusVerifying).Error; err != nil {
		t.Fatalf("set verifying status: %v", err)
	}

	err = imageService.Remove(ctx, testUser.ID, begin.Image.ID)
	if !errors.Is(err, services.ErrInvalidImageState) {
		t.Fatalf("Remove() error = %v, want invalid state", err)
	}
}

func TestImageServiceRejectsOrderedListingImageRemoval(t *testing.T) {
	ctx := context.Background()
	listingService := services.NewListingService(db)
	listing, err := listingService.Create(ctx, services.CreateListingRequest{
		Title: "Ordered listing", Description: "Description",
		Price: 10, SellerID: testUser.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store := &fakeObjectStore{
		authorization: services.UploadAuthorization{URL: "https://upload.example"},
		metadata:      services.ObjectMetadata{SizeBytes: 5, MimeType: "image/png"},
	}
	verifier := &fakeImageVerifier{result: utils.VerifiedImage{
		SizeBytes: 5, MimeType: "image/png", Width: 10, Height: 10,
		ChecksumSHA256: strings.Repeat("d", 64),
	}}
	imageService := services.NewImageService(db, store, verifier)
	begin, err := imageService.BeginUpload(ctx, testUser.ID, services.BeginImageUploadRequest{
		ListingID: listingID(listing.ID), ExpectedSize: 5,
		ExpectedMimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginUpload() error = %v", err)
	}
	if _, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	order := models.Order{
		BuyerID: testUser.ID, ListingID: listing.ID, Title: listing.Title,
		FirstImageID: &begin.Image.ID, Status: "Completed", PurchasedAt: time.Now().UTC(),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	if err := imageService.Remove(
		ctx,
		testUser.ID,
		begin.Image.ID,
	); !errors.Is(err, services.ErrInvalidImageState) {
		t.Fatalf("Remove() error = %v, want invalid state", err)
	}
}

func listingID(id uuid.UUID) *uuid.UUID {
	return &id
}
