package main

import (
	"context"
	"log"

	"github.com/Jcorrieri/uf-marketplace/backend/app"
	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/database"
	"github.com/Jcorrieri/uf-marketplace/backend/objectstore"
	"github.com/Jcorrieri/uf-marketplace/backend/services"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.OpenSQLite(configuration.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}

	store := services.ObjectStore(services.UnavailableObjectStore{})
	if configuration.ObjectStoreProvider == "s3" {
		awsConfiguration, err := awsconfig.LoadDefaultConfig(
			context.Background(),
			awsconfig.WithRegion(configuration.AWSRegion),
		)
		if err != nil {
			log.Fatal(err)
		}
		store = objectstore.NewS3Store(
			s3.NewFromConfig(awsConfiguration),
			configuration.S3Bucket,
		)
	}

	router := app.NewRouterWithObjectStore(db, configuration, store)
	if err := router.Run(configuration.ServerAddress); err != nil {
		log.Fatal(err)
	}
}
