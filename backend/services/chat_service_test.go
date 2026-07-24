package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/database"
	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type chatTestFixture struct {
	service *services.ChatService
	db      *gorm.DB
	buyer   models.User
	seller  models.User
	listing models.Listing
}

func newChatTestFixture(t *testing.T) chatTestFixture {
	t.Helper()

	databaseName := "file:chat_service_" + uuid.NewString() + "?mode=memory&cache=shared"
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
	buyer := models.User{
		Email:        uuid.NewString() + "@ufl.edu",
		PasswordHash: "hash",
		FirstName:    "Buyer",
		LastName:     "Student",
	}
	seller := models.User{
		Email:        uuid.NewString() + "@ufl.edu",
		PasswordHash: "hash",
		FirstName:    "Seller",
		LastName:     "Student",
	}
	if err := gorm.G[models.User](testDB).Create(ctx, &buyer); err != nil {
		t.Fatalf("create buyer: %v", err)
	}
	if err := gorm.G[models.User](testDB).Create(ctx, &seller); err != nil {
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

	return chatTestFixture{
		service: services.NewChatService(testDB),
		db:      testDB,
		buyer:   buyer,
		seller:  seller,
		listing: listing,
	}
}

func TestGetOrCreateConversationDerivesSellerAndReusesConversation(t *testing.T) {
	fixture := newChatTestFixture(t)
	ctx := context.Background()

	first, err := fixture.service.GetOrCreateConversation(
		ctx,
		fixture.buyer.ID,
		fixture.listing.ID,
	)
	if err != nil {
		t.Fatalf("GetOrCreateConversation() error = %v", err)
	}
	if first.SellerID != fixture.seller.ID {
		t.Errorf("SellerID = %s, want %s", first.SellerID, fixture.seller.ID)
	}

	second, err := fixture.service.GetOrCreateConversation(
		ctx,
		fixture.buyer.ID,
		fixture.listing.ID,
	)
	if err != nil {
		t.Fatalf("second GetOrCreateConversation() error = %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second conversation ID = %s, want %s", second.ID, first.ID)
	}

	var conversationCount int64
	if err := fixture.db.Model(&models.Conversation{}).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 1 {
		t.Errorf("conversation count = %d, want 1", conversationCount)
	}
}

func TestGetOrCreateConversationReturnsMissingListing(t *testing.T) {
	fixture := newChatTestFixture(t)

	_, err := fixture.service.GetOrCreateConversation(
		context.Background(),
		fixture.buyer.ID,
		uuid.New(),
	)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestGetOrCreateConversationRejectsListingSeller(t *testing.T) {
	fixture := newChatTestFixture(t)

	_, err := fixture.service.GetOrCreateConversation(
		context.Background(),
		fixture.seller.ID,
		fixture.listing.ID,
	)
	if !errors.Is(err, services.ErrCannotMessageSelf) {
		t.Fatalf("error = %v, want %v", err, services.ErrCannotMessageSelf)
	}
}

func TestSaveMessageReturnsPersistedMessageWithSender(t *testing.T) {
	fixture := newChatTestFixture(t)
	ctx := context.Background()
	conversation, err := fixture.service.GetOrCreateConversation(
		ctx,
		fixture.buyer.ID,
		fixture.listing.ID,
	)
	if err != nil {
		t.Fatalf("GetOrCreateConversation() error = %v", err)
	}

	message := &models.Message{
		ConversationID: conversation.ID,
		SenderID:       fixture.buyer.ID,
		Content:        "Is this available?",
	}
	savedMessage, err := fixture.service.SaveMessage(ctx, message)
	if err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}
	if savedMessage.ID == uuid.Nil {
		t.Error("saved message ID is empty")
	}
	if savedMessage.Content != message.Content {
		t.Errorf("Content = %q, want %q", savedMessage.Content, message.Content)
	}
	if savedMessage.CreatedAt.IsZero() {
		t.Error("saved message CreatedAt is empty")
	}
	if savedMessage.Sender.ID != fixture.buyer.ID {
		t.Errorf("Sender.ID = %s, want %s", savedMessage.Sender.ID, fixture.buyer.ID)
	}
	if savedMessage.Sender.FirstName != fixture.buyer.FirstName {
		t.Errorf(
			"Sender.FirstName = %q, want %q",
			savedMessage.Sender.FirstName,
			fixture.buyer.FirstName,
		)
	}
}
