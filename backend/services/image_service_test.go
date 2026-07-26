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

func (store *fakeObjectStore) Stat(
	context.Context,
	string,
) (services.ObjectMetadata, error) {
	return store.metadata, store.statErr
}

func (store *fakeObjectStore) Open(context.Context, string) (io.ReadCloser, error) {
	if store.openErr != nil {
		return nil, store.openErr
	}
	return io.NopCloser(strings.NewReader("image")), nil
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
	if !strings.Contains(store.uploadRequest.ObjectKey, listing.ID.String()) {
		t.Errorf("object key %q does not contain listing ID", store.uploadRequest.ObjectKey)
	}

	ready, err := imageService.CompleteUpload(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if ready.Status != models.ImageStatusReady || ready.ChecksumSHA256 == nil {
		t.Fatalf("completed image = %#v", ready)
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

func TestImageServiceMarksFailedVerification(t *testing.T) {
	ctx := context.Background()
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
		ExpectedSize: 5, ExpectedMimeType: "image/png",
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

func TestImageServiceDetachesBeforeDeletion(t *testing.T) {
	ctx := context.Background()
	listingService := services.NewListingService(db)
	listing, err := listingService.Create(ctx, services.CreateListingRequest{
		Title: "Detach listing", Description: "Description", Price: 10, SellerID: testUser.ID,
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

	if err := imageService.MarkForDeletion(
		ctx,
		testUser.ID,
		begin.Image.ID,
	); !errors.Is(err, services.ErrImageStillReferenced) {
		t.Fatalf("MarkForDeletion() error = %v, want referenced", err)
	}
	if err := imageService.Detach(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("Detach() error = %v", err)
	}
	if err := imageService.MarkForDeletion(ctx, testUser.ID, begin.Image.ID); err != nil {
		t.Fatalf("MarkForDeletion() after detach error = %v", err)
	}
	image, err := imageService.GetMetadata(ctx, testUser.ID, begin.Image.ID)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if image.Status != models.ImageStatusDeleting || image.DetachedAt == nil {
		t.Errorf("image = %#v, want detached and deleting", image)
	}
}

func listingID(id uuid.UUID) *uuid.UUID {
	return &id
}
