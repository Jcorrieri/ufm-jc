package services_test

import (
	"testing"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/google/uuid"
)

const hubTestTimeout = time.Second

func newRunningHub() *services.Hub {
	hub := services.NewHub()
	go hub.Run()
	return hub
}

func receiveHubMessage(t *testing.T, messages <-chan []byte) []byte {
	t.Helper()

	select {
	case message, open := <-messages:
		if !open {
			t.Fatal("subscription closed before receiving a message")
		}
		return message
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for hub message")
		return nil
	}
}

func TestHubPublishesToEverySubscriberInRoom(t *testing.T) {
	hub := newRunningHub()
	conversationID := uuid.New()
	firstMessages, unsubscribeFirst := hub.Subscribe(conversationID)
	secondMessages, unsubscribeSecond := hub.Subscribe(conversationID)
	t.Cleanup(unsubscribeFirst)
	t.Cleanup(unsubscribeSecond)

	hub.Publish(conversationID, []byte("hello"))

	if message := string(receiveHubMessage(t, firstMessages)); message != "hello" {
		t.Errorf("first message = %q, want %q", message, "hello")
	}
	if message := string(receiveHubMessage(t, secondMessages)); message != "hello" {
		t.Errorf("second message = %q, want %q", message, "hello")
	}
}

func TestHubKeepsConversationRoomsIsolated(t *testing.T) {
	hub := newRunningHub()
	firstMessages, unsubscribeFirst := hub.Subscribe(uuid.New())
	secondConversationID := uuid.New()
	secondMessages, unsubscribeSecond := hub.Subscribe(secondConversationID)
	t.Cleanup(unsubscribeFirst)
	t.Cleanup(unsubscribeSecond)

	hub.Publish(secondConversationID, []byte("second room"))

	if message := string(receiveHubMessage(t, secondMessages)); message != "second room" {
		t.Errorf("message = %q, want %q", message, "second room")
	}
	select {
	case message := <-firstMessages:
		t.Fatalf("first room unexpectedly received %q", message)
	default:
	}
}

func TestHubUnsubscribeClosesSubscriptionAndIsIdempotent(t *testing.T) {
	hub := newRunningHub()
	messages, unsubscribe := hub.Subscribe(uuid.New())

	unsubscribe()
	unsubscribe()

	select {
	case _, open := <-messages:
		if open {
			t.Fatal("subscription remained open after unsubscribe")
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for subscription to close")
	}
}

func TestHubRemovesSubscriberWhenItsBufferIsFull(t *testing.T) {
	hub := newRunningHub()
	conversationID := uuid.New()
	messages, unsubscribe := hub.Subscribe(conversationID)
	t.Cleanup(unsubscribe)

	for index := 0; index < 512; index++ {
		hub.Publish(conversationID, []byte("message"))
	}

	for {
		select {
		case _, open := <-messages:
			if !open {
				return
			}
		case <-time.After(hubTestTimeout):
			t.Fatal("timed out waiting for slow subscription to close")
		}
	}
}
