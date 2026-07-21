package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/config"
	"github.com/Jcorrieri/uf-marketplace/backend/database"
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

	imageSource := database.PicsumImageSource{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
	if err := database.Seed(context.Background(), db, imageSource); err != nil {
		log.Fatal(err)
	}
}
