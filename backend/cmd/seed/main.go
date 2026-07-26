package main

import (
	"context"
	"log"

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

	if err := database.Seed(context.Background(), db); err != nil {
		log.Fatal(err)
	}
}
