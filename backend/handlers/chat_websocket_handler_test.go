package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/handlers"
	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type fakeWebSocketChatService struct {
	mutex        sync.Mutex
	conversation models.Conversation
	getByIDError error
	savedInput   models.Message
	savedMessage models.Message
	saveError    error
}

func (service *fakeWebSocketChatService) GetByID(
	context.Context,
	uuid.UUID,
) (models.Conversation, error) {
	return service.conversation, service.getByIDError
}

func (service *fakeWebSocketChatService) SaveMessage(
	_ context.Context,
	message *models.Message,
) (*models.Message, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.savedInput = *message
	if service.saveError != nil {
		return nil, service.saveError
	}
	savedMessage := service.savedMessage
	return &savedMessage, nil
}

func (service *fakeWebSocketChatService) getSavedInput() models.Message {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	return service.savedInput
}

func newWebSocketTestRouter(
	userID uuid.UUID,
	handler *handlers.ChatWebSocketHandler,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws/:id", func(c *gin.Context) {
		c.Set("userID", userID.String())
		handler.Serve(c)
	})
	return router
}

func TestChatWebSocketHandlerRejectsNonParticipantBeforeUpgrade(t *testing.T) {
	conversationID := uuid.New()
	service := &fakeWebSocketChatService{
		conversation: models.Conversation{
			ID:       conversationID,
			BuyerID:  uuid.New(),
			SellerID: uuid.New(),
		},
	}
	handler := handlers.NewChatWebSocketHandler(service, services.NewHub())
	router := newWebSocketTestRouter(uuid.New(), handler)
	request := httptest.NewRequest(http.MethodGet, "/ws/"+conversationID.String(), nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestChatWebSocketHandlerReturnsNotFoundBeforeUpgrade(t *testing.T) {
	service := &fakeWebSocketChatService{getByIDError: errors.New("not found")}
	handler := handlers.NewChatWebSocketHandler(service, services.NewHub())
	router := newWebSocketTestRouter(uuid.New(), handler)
	request := httptest.NewRequest(http.MethodGet, "/ws/"+uuid.NewString(), nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestChatWebSocketHandlerSavesAndBroadcastsMessage(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	messageID := uuid.New()
	createdAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	service := &fakeWebSocketChatService{
		conversation: models.Conversation{
			ID:       conversationID,
			BuyerID:  userID,
			SellerID: uuid.New(),
		},
		savedMessage: models.Message{
			ID:             messageID,
			ConversationID: conversationID,
			SenderID:       userID,
			Sender: models.User{
				ID:        userID,
				FirstName: "Test",
				LastName:  "Buyer",
			},
			Content:   "Is this available?",
			CreatedAt: createdAt,
		},
	}
	hub := services.NewHub()
	go hub.Run()
	handler := handlers.NewChatWebSocketHandler(service, hub)
	server := httptest.NewServer(newWebSocketTestRouter(userID, handler))
	t.Cleanup(server.Close)

	webSocketURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/ws/" + conversationID.String()
	connection, _, err := websocket.DefaultDialer.Dial(webSocketURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	if err := connection.WriteMessage(
		websocket.TextMessage,
		[]byte("Is this available?"),
	); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	var response models.MessageResponse
	if err := connection.ReadJSON(&response); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if response.ID != messageID {
		t.Errorf("response ID = %s, want %s", response.ID, messageID)
	}
	if response.SenderName != "Test Buyer" {
		t.Errorf("sender name = %q, want %q", response.SenderName, "Test Buyer")
	}
	if response.Content != "Is this available?" {
		t.Errorf("content = %q, want %q", response.Content, "Is this available?")
	}
	if !response.CreatedAt.Equal(createdAt) {
		t.Errorf("created at = %v, want %v", response.CreatedAt, createdAt)
	}

	savedInput := service.getSavedInput()
	if savedInput.ConversationID != conversationID {
		t.Errorf(
			"saved conversation ID = %s, want %s",
			savedInput.ConversationID,
			conversationID,
		)
	}
	if savedInput.SenderID != userID {
		t.Errorf("saved sender ID = %s, want %s", savedInput.SenderID, userID)
	}
	if savedInput.Content != "Is this available?" {
		t.Errorf("saved content = %q, want %q", savedInput.Content, "Is this available?")
	}
}
