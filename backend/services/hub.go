package services

import (
	"sync"

	"github.com/google/uuid"
)

const subscriberBufferSize = 256

type subscription struct {
	conversationID uuid.UUID
	messages       chan []byte
}

type broadcastMessage struct {
	conversationID uuid.UUID
	data           []byte
}

// Hub manages in-memory subscriptions grouped by conversation.
type Hub struct {
	rooms      map[uuid.UUID]map[chan []byte]struct{}
	broadcast  chan broadcastMessage
	register   chan subscription
	unregister chan subscription
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[chan []byte]struct{}),
		broadcast:  make(chan broadcastMessage),
		register:   make(chan subscription),
		unregister: make(chan subscription),
	}
}

// Subscribe registers a buffered message channel for a conversation.
// The returned unsubscribe function is safe to call more than once.
func (h *Hub) Subscribe(conversationID uuid.UUID) (<-chan []byte, func()) {
	messages := make(chan []byte, subscriberBufferSize)
	subscription := subscription{
		conversationID: conversationID,
		messages:       messages,
	}
	h.register <- subscription

	var unsubscribeOnce sync.Once
	unsubscribe := func() {
		unsubscribeOnce.Do(func() {
			h.unregister <- subscription
		})
	}

	return messages, unsubscribe
}

// Publish sends a message to every active subscriber in a conversation.
func (h *Hub) Publish(conversationID uuid.UUID, data []byte) {
	h.broadcast <- broadcastMessage{
		conversationID: conversationID,
		data:           data,
	}
}

// Run processes subscriptions and broadcasts. Call it once at startup.
func (h *Hub) Run() {
	for {
		select {
		case subscriber := <-h.register:
			room := subscriber.conversationID
			if h.rooms[room] == nil {
				h.rooms[room] = make(map[chan []byte]struct{})
			}
			h.rooms[room][subscriber.messages] = struct{}{}

		case subscriber := <-h.unregister:
			h.removeSubscriber(subscriber)

		case message := <-h.broadcast:
			for messages := range h.rooms[message.conversationID] {
				select {
				case messages <- message.data:
				default:
					close(messages)
					delete(h.rooms[message.conversationID], messages)
				}
			}
			if len(h.rooms[message.conversationID]) == 0 {
				delete(h.rooms, message.conversationID)
			}
		}
	}
}

func (h *Hub) removeSubscriber(subscriber subscription) {
	room := h.rooms[subscriber.conversationID]
	if _, exists := room[subscriber.messages]; !exists {
		return
	}

	delete(room, subscriber.messages)
	close(subscriber.messages)
	if len(room) == 0 {
		delete(h.rooms, subscriber.conversationID)
	}
}
