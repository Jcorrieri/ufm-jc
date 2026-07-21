package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const seedMarkerEmail = "jsmack@ufl.edu"

var seedImageURLs = []string{
	"https://picsum.photos/id/10/400/300.jpg",
	"https://picsum.photos/id/20/400/300.jpg",
	"https://picsum.photos/id/30/400/300.jpg",
	"https://picsum.photos/id/40/400/300.jpg",
	"https://picsum.photos/id/50/400/300.jpg",
}

type SeedImage struct {
	Data     []byte
	MimeType string
}

// ImageSource supplies the images attached to seed listings.
type ImageSource interface {
	Images(context.Context) ([]SeedImage, error)
}

// PicsumImageSource downloads the standard development seed images from Picsum.
type PicsumImageSource struct {
	Client *http.Client
}

func (source PicsumImageSource) Images(ctx context.Context) ([]SeedImage, error) {
	client := source.Client
	if client == nil {
		client = http.DefaultClient
	}

	images := make([]SeedImage, 0, len(seedImageURLs))
	for _, imageURL := range seedImageURLs {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create seed image request for %s: %w", imageURL, err)
		}

		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("download seed image %s: %w", imageURL, err)
		}

		data, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf(
				"download seed image %s: unexpected status %s",
				imageURL,
				response.Status,
			)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read seed image %s: %w", imageURL, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close seed image response %s: %w", imageURL, closeErr)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("download seed image %s: response was empty", imageURL)
		}

		mimeType := response.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		images = append(images, SeedImage{Data: data, MimeType: mimeType})
	}

	return images, nil
}

// Seed inserts development fixtures atomically. It does nothing if fixtures already exist.
func Seed(ctx context.Context, db *gorm.DB, imageSource ImageSource) error {
	if imageSource == nil {
		return errors.New("seed database: image source is required")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userService := services.NewUserService(tx)
		_, err := userService.GetByEmail(ctx, seedMarkerEmail)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing seed data: %w", err)
		}

		images, err := imageSource.Images(ctx)
		if err != nil {
			return fmt.Errorf("load seed images: %w", err)
		}
		if len(images) == 0 {
			return errors.New("load seed images: image source returned no images")
		}
		for index, image := range images {
			if len(image.Data) == 0 || image.MimeType == "" {
				return fmt.Errorf("load seed images: image %d is incomplete", index)
			}
		}

		users, err := seedUsers(ctx, userService)
		if err != nil {
			return err
		}
		if err := seedListings(ctx, services.NewListingService(tx), users, images); err != nil {
			return err
		}
		return nil
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
	seedImages []SeedImage,
) error {
	images := make([]models.Image, len(seedImages))
	for index, image := range seedImages {
		images[index] = models.Image{Data: image.Data, MimeType: image.MimeType}
	}

	listings := []*models.Listing{
		seedListing("Standing Desk", "Adjustable standing desk, great condition. Perfect for studying.",
			85, users[0].ID, images[0]),
		seedListing("Mountain Bike", "Trek mountain bike, barely used. Includes lock and helmet.",
			220, users[3].ID, images[1%len(images)]),
		seedListing("Organic Chemistry Textbook", "8th edition, no highlights. ISBN 978-0134042282.",
			45, users[2].ID, images[2%len(images)]),
		seedListing("27\" Monitor", "Dell 27\" 1440p IPS monitor. Comes with HDMI cable.",
			150, users[3].ID, images[3%len(images)]),
		seedListing("Futon Couch", "Foldable futon, dark grey. Great for dorm rooms.",
			60, users[4].ID, images[4%len(images)]),
		seedListing("Acoustic Guitar", "Yamaha FG800, excellent sound. Includes gig bag and tuner.",
			130, users[5].ID, images[5%len(images)]),
		seedListing("Desk Lamp", "LED desk lamp with USB charging port. 3 brightness levels.",
			18, users[6].ID, images[6%len(images)]),
		seedListing("North Face Backpack", "Black Borealis backpack, very spacious. Minor wear.",
			40, users[7].ID, images[7%len(images)]),
	}

	const generatedListingCount = 80
	for index := range generatedListingCount {
		number := index + len(listings) + 1
		listings = append(listings, seedListing(
			fmt.Sprintf("Product %d", number),
			"This is a sample description for the item.",
			float64(20+(number-1)*2),
			users[(number-1)%len(users)].ID,
			images[(number-1)%len(images)],
		))
	}

	for _, listing := range listings {
		if err := listingService.Create(ctx, listing); err != nil {
			return fmt.Errorf("create seed listing %q: %w", listing.Title, err)
		}
	}
	return nil
}

func seedListing(
	title string,
	description string,
	price float64,
	sellerID uuid.UUID,
	image models.Image,
) *models.Listing {
	return &models.Listing{
		Title: title, Description: description, Price: price, SellerID: sellerID,
		Images: []models.Image{image},
	}
}
