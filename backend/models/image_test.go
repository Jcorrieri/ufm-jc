package models_test

import (
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/google/uuid"
)

func TestImageBeforeCreatePreservesPreassignedID(t *testing.T) {
	imageID := uuid.New()
	image := models.Image{ID: imageID}

	if err := image.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate returned an unexpected error: %v", err)
	}
	if image.ID != imageID {
		t.Errorf("ID = %v, want preassigned ID %v", image.ID, imageID)
	}
}

func TestImageBeforeCreateAssignsUUID(t *testing.T) {
	image := models.Image{}

	if err := image.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate returned an unexpected error: %v", err)
	}
	if image.ID == uuid.Nil {
		t.Error("BeforeCreate did not assign an ID")
	}
}

func TestImageResponseIncludesExpectedMetadata(t *testing.T) {
	image := models.Image{
		ExpectedSizeBytes: 42,
		ExpectedMimeType:  "image/png",
	}

	response := image.GetResponse()

	if response.ExpectedSizeBytes != image.ExpectedSizeBytes {
		t.Errorf("ExpectedSizeBytes = %d, want %d", response.ExpectedSizeBytes, 42)
	}
	if response.ExpectedMimeType != image.ExpectedMimeType {
		t.Errorf("ExpectedMimeType = %q, want image/png", response.ExpectedMimeType)
	}
}
