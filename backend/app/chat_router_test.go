package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/app"
	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/database"
	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/Jcorrieri/uf-marketplace/backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	chatRouterJWTSecret  = "chat-router-test-secret"
	chatRouterCookieName = "chat_router_session"
)

type chatRouterFixture struct {
	router  *gin.Engine
	listing models.Listing
	buyer   models.User
	seller  models.User
}

func newChatRouterFixture(t *testing.T) chatRouterFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	databaseName := "file:chat_router_" + uuid.NewString() + "?mode=memory&cache=shared"
	testDB, err := database.OpenSQLite(databaseName)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := database.Migrate(testDB); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	ctx := context.Background()
	userService := services.NewUserService(testDB)
	buyer, err := userService.Create(ctx, services.CreateUserRequest{
		Email:     uuid.NewString() + "@ufl.edu",
		Password:  "password",
		FirstName: "Buyer",
		LastName:  "Student",
	})
	if err != nil {
		t.Fatalf("create buyer: %v", err)
	}
	seller, err := userService.Create(ctx, services.CreateUserRequest{
		Email:     uuid.NewString() + "@ufl.edu",
		Password:  "password",
		FirstName: "Seller",
		LastName:  "Student",
	})
	if err != nil {
		t.Fatalf("create seller: %v", err)
	}

	listing := models.Listing{
		Title:       "Desk",
		Description: "A sturdy desk",
		Price:       25,
		SellerID:    seller.ID,
	}
	if err := gorm.G[models.Listing](testDB).Create(ctx, &listing); err != nil {
		t.Fatalf("create listing: %v", err)
	}

	router := app.NewRouter(testDB, config.Config{
		ServerAddress:     "localhost:8080",
		DatabasePath:      "unused.db",
		JWTSecret:         chatRouterJWTSecret,
		SessionCookieName: chatRouterCookieName,
	})

	return chatRouterFixture{
		router:  router,
		listing: listing,
		buyer:   *buyer,
		seller:  *seller,
	}
}

func (fixture chatRouterFixture) startConversation(
	t *testing.T,
	userID uuid.UUID,
	listingID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]string{"listing_id": listingID.String()})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	token, err := utils.GenerateToken(userID, chatRouterJWTSecret)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/conversations",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: chatRouterCookieName, Value: token})

	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	return response
}

func TestStartConversationDerivesSellerFromListing(t *testing.T) {
	fixture := newChatRouterFixture(t)
	response := fixture.startConversation(t, fixture.buyer.ID, fixture.listing.ID)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body)
	}

	var conversation models.ConversationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &conversation); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if conversation.SellerID != fixture.seller.ID {
		t.Errorf("SellerID = %s, want %s", conversation.SellerID, fixture.seller.ID)
	}
}

func TestStartConversationReturnsNotFoundForMissingListing(t *testing.T) {
	fixture := newChatRouterFixture(t)
	response := fixture.startConversation(t, fixture.buyer.ID, uuid.New())

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusNotFound,
			response.Body,
		)
	}
}

func TestStartConversationRejectsListingSeller(t *testing.T) {
	fixture := newChatRouterFixture(t)
	response := fixture.startConversation(t, fixture.seller.ID, fixture.listing.ID)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusBadRequest,
			response.Body,
		)
	}
}
