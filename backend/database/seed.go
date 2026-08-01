package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"gorm.io/gorm"
)

const seedMarkerEmail = "jsmack@ufl.edu"

// Seed inserts image-free development fixtures atomically.
// Object-backed seed images will be added with the object-store adapter.
func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userService := services.NewUserService(tx)
		_, err := userService.GetByEmail(ctx, seedMarkerEmail)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing seed data: %w", err)
		}

		users, err := seedUsers(ctx, userService)
		if err != nil {
			return err
		}
		return seedListings(ctx, services.NewListingService(tx), users)
	})
}

func seedUsers(
	ctx context.Context,
	userService *services.UserService,
) ([]*models.User, error) {
	requests := []services.CreateUserRequest{
		{Email: "test@ufl.edu", Password: "password", FirstName: "Test", LastName: "User"},
		{Email: seedMarkerEmail, Password: "password", FirstName: "John", LastName: "Smack"},
		{Email: "adoe@ufl.edu", Password: "password", FirstName: "Alice", LastName: "Doe"},
		{Email: "bsmith@ufl.edu", Password: "password", FirstName: "Bob", LastName: "Smith"},
		{Email: "cjohnson@ufl.edu", Password: "password", FirstName: "Carol", LastName: "Johnson"},
		{Email: "dlee@ufl.edu", Password: "password", FirstName: "David", LastName: "Lee"},
		{Email: "ewalker@ufl.edu", Password: "password", FirstName: "Emma", LastName: "Walker"},
		{Email: "fmartin@ufl.edu", Password: "password", FirstName: "Frank", LastName: "Martin"},
		{Email: "gclark@ufl.edu", Password: "password", FirstName: "Grace", LastName: "Clark"},
		{Email: "hlopez@ufl.edu", Password: "password", FirstName: "Hector", LastName: "Lopez"},
		{Email: "ikim@ufl.edu", Password: "password", FirstName: "Ivy", LastName: "Kim"},
	}

	users := make([]*models.User, 0, len(requests))
	for _, request := range requests {
		user, err := userService.Create(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("create seed user %s: %w", request.Email, err)
		}
		users = append(users, user)
	}
	return users, nil
}

func seedListings(
	ctx context.Context,
	listingService *services.ListingService,
	users []*models.User,
) error {
	listings := []services.CreateListingRequest{
		seedListing("Standing Desk", "Adjustable standing desk, great condition.", 85, users[0]),
		seedListing("Mountain Bike", "Trek mountain bike, barely used.", 220, users[3]),
		seedListing("Organic Chemistry Textbook", "8th edition, no highlights.", 45, users[2]),
		seedListing("27\" Monitor", "Dell 27-inch 1440p IPS monitor.", 150, users[3]),
		seedListing("Futon Couch", "Foldable dark grey futon.", 60, users[4]),
		seedListing("Acoustic Guitar", "Yamaha FG800 with gig bag.", 130, users[5]),
		seedListing("Desk Lamp", "LED lamp with USB charging.", 18, users[6]),
		seedListing("North Face Backpack", "Black Borealis backpack.", 40, users[7]),
	}

	const generatedListingCount = 80
	for index := range generatedListingCount {
		number := index + len(listings) + 1
		listings = append(listings, services.CreateListingRequest{
			Title:       fmt.Sprintf("Product %d", number),
			Description: "This is a sample description for the item.",
			Price:       float64(20 + (number-1)*2),
			SellerID:    users[(number-1)%len(users)].ID,
		})
	}

	for _, request := range listings {
		listing, err := listingService.Create(ctx, request)
		if err != nil {
			return fmt.Errorf("create seed listing %q: %w", request.Title, err)
		}
		if _, err := listingService.Publish(ctx, listing.ID, request.SellerID); err != nil {
			return fmt.Errorf("publish seed listing %q: %w", request.Title, err)
		}
	}
	return nil
}

func seedListing(
	title string,
	description string,
	price float64,
	seller *models.User,
) services.CreateListingRequest {
	return services.CreateListingRequest{
		Title: title, Description: description, Price: price, SellerID: seller.ID,
	}
}
