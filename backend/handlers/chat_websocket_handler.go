package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	webSocketWriteWait      = 10 * time.Second
	webSocketPongWait       = 60 * time.Second
	webSocketPingPeriod     = (webSocketPongWait * 9) / 10
	webSocketMaxMessageSize = 4096
)

type chatWebSocketService interface {
	GetByID(context.Context, uuid.UUID) (models.Conversation, error)
	SaveMessage(context.Context, *models.Message) (*models.Message, error)
}

type ChatWebSocketHandler struct {
	chatService chatWebSocketService
	hub         *services.Hub
	upgrader    websocket.Upgrader
}

func NewChatWebSocketHandler(
	chatService chatWebSocketService,
	hub *services.Hub,
) *ChatWebSocketHandler {
	return &ChatWebSocketHandler{
		chatService: chatService,
		hub:         hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Allow all origins for now — tighten in production.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Serve authorizes and upgrades a WebSocket connection for a conversation.
func (h *ChatWebSocketHandler) Serve(c *gin.Context) {
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	conversation, err := h.chatService.GetByID(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}
	if userID != conversation.BuyerID && userID != conversation.SellerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	connection, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	outgoingMessages, unsubscribe := h.hub.Subscribe(conversationID)
	client := &webSocketClient{
		chatService:    h.chatService,
		hub:            h.hub,
		connection:     connection,
		outgoing:       outgoingMessages,
		unsubscribe:    unsubscribe,
		conversationID: conversationID,
		userID:         userID,
	}

	go client.writePump()
	go client.readPump()
}

type webSocketClient struct {
	chatService    chatWebSocketService
	hub            *services.Hub
	connection     *websocket.Conn
	outgoing       <-chan []byte
	unsubscribe    func()
	conversationID uuid.UUID
	userID         uuid.UUID
}

func (client *webSocketClient) readPump() {
	defer func() {
		client.unsubscribe()
		_ = client.connection.Close()
	}()

	client.connection.SetReadLimit(webSocketMaxMessageSize)
	_ = client.connection.SetReadDeadline(time.Now().Add(webSocketPongWait))
	client.connection.SetPongHandler(func(string) error {
		return client.connection.SetReadDeadline(time.Now().Add(webSocketPongWait))
	})

	for {
		_, rawMessage, err := client.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf("ws error: %v", err)
			}
			return
		}

		content := string(rawMessage)
		if content == "" {
			continue
		}

		message := &models.Message{
			ConversationID: client.conversationID,
			SenderID:       client.userID,
			Content:        content,
		}
		savedMessage, err := client.chatService.SaveMessage(context.Background(), message)
		if err != nil {
			log.Printf("failed to save message: %v", err)
			continue
		}

		data, err := json.Marshal(savedMessage.GetResponse())
		if err != nil {
			log.Printf("failed to encode message: %v", err)
			continue
		}
		client.hub.Publish(client.conversationID, data)
	}
}

func (client *webSocketClient) writePump() {
	ticker := time.NewTicker(webSocketPingPeriod)
	defer func() {
		ticker.Stop()
		_ = client.connection.Close()
	}()

	for {
		select {
		case message, open := <-client.outgoing:
			_ = client.connection.SetWriteDeadline(time.Now().Add(webSocketWriteWait))
			if !open {
				_ = client.connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.connection.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = client.connection.SetWriteDeadline(time.Now().Add(webSocketWriteWait))
			if err := client.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
