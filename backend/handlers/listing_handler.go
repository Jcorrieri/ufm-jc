package handlers

import (
	"net/http"
	"strconv"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ListingHandler struct {
	listingService *services.ListingService
}

func NewListingHandler(s *services.ListingService) *ListingHandler {
	return &ListingHandler{listingService: s}
}

// GET /api/listings
func (h *ListingHandler) GetListings(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter."})
		return
	}

	cursor, err := uuid.Parse(c.Query("cursor")) // UUID string, empty or "0" means no cursor
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cursor parameter."})
		return
	}

	var listings []models.Listing

	query := c.Query("query")
	if query != "" {
		listings, err = h.listingService.Search(c.Request.Context(), query, limit, cursor)
	} else {
		listings, err = h.listingService.GetAll(c.Request.Context(), limit, cursor)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch listings."})
		return
	}

	var response []models.ListingResponse
	for _, l := range listings {
		response = append(response, l.GetResponse())
	}

	c.JSON(http.StatusOK, response)
}

type listingInput struct {
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description" binding:"required"`
	Price       float64 `json:"price" binding:"gte=0"`
}

// POST /api/listings
func (h *ListingHandler) CreateListing(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input listingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid listing"})
		return
	}

	listing, err := h.listingService.Create(
		c.Request.Context(),
		services.CreateListingRequest{
			Title:       input.Title,
			Description: input.Description,
			Price:       input.Price,
			SellerID:    userID,
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create listing"})
		return
	}

	c.JSON(http.StatusCreated, listing.GetResponse())
}

// GET /api/listings/me
func (h *ListingHandler) GetMyListings(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	listings, err := h.listingService.GetBySellerID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch listings."})
		return
	}

	var response []models.ListingResponse
	for _, l := range listings {
		response = append(response, l.GetResponse())
	}

	c.JSON(http.StatusOK, response)
}

// PUT /api/listings/:id
func (h *ListingHandler) UpdateListing(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	listing, err := h.listingService.GetByID(c.Request.Context(), listingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Listing not found"})
		return
	}

	if userID != listing.SellerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	var input listingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid listing"})
		return
	}

	updated, err := h.listingService.Update(
		c.Request.Context(),
		listingID,
		services.UpdateListingRequest{
			Title:       input.Title,
			Description: input.Description,
			Price:       input.Price,
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated.GetResponse())
}

// POST /api/listings/:id/publish
func (h *ListingHandler) PublishListing(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	listing, err := h.listingService.Publish(c.Request.Context(), listingID, userID)
	if err != nil {
		status := http.StatusConflict
		if err == services.ErrInvalidImageState {
			c.JSON(status, gin.H{"error": "Listing has unresolved images"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Draft listing not found"})
		return
	}
	c.JSON(http.StatusOK, listing.GetResponse())
}

// DELETE /api/listings/:id
func (h *ListingHandler) DeleteListing(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
		return
	}

	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid listing ID"})
		return
	}

	if err := h.listingService.Delete(
		c.Request.Context(),
		listingID,
		userID,
	); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Listing not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete listing"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Listing deleted"})
}

// DELETE /api/listing-drafts/:id
func (h *ListingHandler) AbortDraft(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	listingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid listing ID"})
		return
	}
	err = h.listingService.AbortDraft(c.Request.Context(), listingID, userID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case err == services.ErrInvalidImageState:
		c.JSON(http.StatusConflict, gin.H{"error": "Listing is no longer a draft"})
	case err == gorm.ErrRecordNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "Draft listing not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to abort draft"})
	}
}
