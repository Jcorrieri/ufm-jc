package handlers

import (
	"errors"
	"net/http"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImageHandler struct {
	imageService *services.ImageService
}

func NewImageHandler(imageService *services.ImageService) *ImageHandler {
	return &ImageHandler{imageService: imageService}
}

type beginImageUploadInput struct {
	ListingID        *uuid.UUID `json:"listing_id"`
	ExpectedSize     int64      `json:"expected_size"`
	ExpectedMimeType string     `json:"expected_mime_type"`
	Position         int        `json:"position"`
}

type beginImageUploadResponse struct {
	Image         models.ImageResponse         `json:"image"`
	Authorization services.UploadAuthorization `json:"authorization"`
}

// POST /api/images/uploads
func (h *ImageHandler) BeginUpload(c *gin.Context) {
	actorID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var input beginImageUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image metadata"})
		return
	}

	result, err := h.imageService.BeginUpload(
		c.Request.Context(),
		actorID,
		services.BeginImageUploadRequest{
			ListingID:        input.ListingID,
			ExpectedSize:     input.ExpectedSize,
			ExpectedMimeType: input.ExpectedMimeType,
			Position:         input.Position,
		},
	)
	if err != nil {
		writeImageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, beginImageUploadResponse{
		Image:         result.Image.GetResponse(),
		Authorization: result.Authorization,
	})
}

// POST /api/images/:imageId/complete
func (h *ImageHandler) CompleteUpload(c *gin.Context) {
	actorID, imageID, ok := imageRequestIDs(c)
	if !ok {
		return
	}
	image, err := h.imageService.CompleteUpload(c.Request.Context(), actorID, imageID)
	if err != nil {
		writeImageError(c, err)
		return
	}
	c.JSON(http.StatusOK, image.GetResponse())
}

// GET /api/images/:imageId/status
func (h *ImageHandler) GetStatus(c *gin.Context) {
	actorID, imageID, ok := imageRequestIDs(c)
	if !ok {
		return
	}
	image, err := h.imageService.GetMetadata(c.Request.Context(), actorID, imageID)
	if err != nil {
		writeImageError(c, err)
		return
	}
	c.JSON(http.StatusOK, image.GetResponse())
}

// POST /api/images/:imageId/detach
func (h *ImageHandler) DetachImage(c *gin.Context) {
	actorID, imageID, ok := imageRequestIDs(c)
	if !ok {
		return
	}
	if err := h.imageService.Detach(c.Request.Context(), actorID, imageID); err != nil {
		writeImageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DELETE /api/images/:imageId
func (h *ImageHandler) DeleteImage(c *gin.Context) {
	actorID, imageID, ok := imageRequestIDs(c)
	if !ok {
		return
	}
	if err := h.imageService.MarkForDeletion(c.Request.Context(), actorID, imageID); err != nil {
		writeImageError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// GET /api/images/:imageId
func (h *ImageHandler) GetImage(c *gin.Context) {
	imageID, err := uuid.Parse(c.Param("imageId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}
	url, err := h.imageService.GetDownloadURL(c.Request.Context(), imageID)
	if err != nil {
		writeImageError(c, err)
		return
	}
	c.Redirect(http.StatusFound, url)
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	actorID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return uuid.Nil, false
	}
	return actorID, true
}

func imageRequestIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authenticatedUserID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	imageID, err := uuid.Parse(c.Param("imageId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, imageID, true
}

func writeImageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrObjectStoreUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Object storage unavailable"})
	case errors.Is(err, services.ErrInvalidImageMetadata):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid image metadata"})
	case errors.Is(err, services.ErrImageVerification):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Image verification failed"})
	case errors.Is(err, services.ErrInvalidImageState):
		c.JSON(http.StatusConflict, gin.H{"error": "Invalid image state"})
	case errors.Is(err, services.ErrImageStillReferenced):
		c.JSON(http.StatusConflict, gin.H{"error": "Image is still referenced"})
	case errors.Is(err, services.ErrImageNotOwned), errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Image operation failed"})
	}
}
