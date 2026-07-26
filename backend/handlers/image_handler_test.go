package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/database"
	"github.com/Jcorrieri/uf-marketplace/backend/handlers"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newImageHandlerTestRouter(t *testing.T) (*gin.Engine, uuid.UUID) {
	t.Helper()
	db, err := database.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	user, err := services.NewUserService(db).Create(
		context.Background(),
		services.CreateUserRequest{
			Email: "image-handler@ufl.edu", Password: "password",
			FirstName: "Image", LastName: "Handler",
		},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	imageService := services.NewImageService(
		db,
		services.UnavailableObjectStore{},
		utils.NewStandardImageVerifier(4096, 4096, 16*1024*1024),
	)
	handler := handlers.NewImageHandler(imageService)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/images/uploads", func(c *gin.Context) {
		c.Set("userID", user.ID.String())
		handler.BeginUpload(c)
	})
	router.POST("/images/:imageId/detach", func(c *gin.Context) {
		c.Set("userID", user.ID.String())
		handler.DetachImage(c)
	})
	return router, user.ID
}

func TestImageHandlerRejectsInvalidUploadMetadata(t *testing.T) {
	router, _ := newImageHandlerTestRouter(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/images/uploads",
		bytes.NewBufferString(
			`{"expected_size":0,"expected_mime_type":"image/gif","position":0}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestImageHandlerReportsUnavailableObjectStore(t *testing.T) {
	router, _ := newImageHandlerTestRouter(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/images/uploads",
		bytes.NewBufferString(
			`{"expected_size":100,"expected_mime_type":"image/png","position":0}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestImageHandlerRejectsInvalidDetachID(t *testing.T) {
	router, _ := newImageHandlerTestRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/images/not-a-uuid/detach", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
