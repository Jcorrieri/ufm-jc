package database

import (
	"fmt"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenSQLite opens a SQLite database without changing its schema or data.
func OpenSQLite(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", path, err)
	}

	return db, nil
}

// Migrate applies the application's schema migrations.
func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.User{},
		&models.PasswordResetToken{},
		&models.Listing{},
		&models.Image{},
		&models.Order{},
		&models.Conversation{},
		&models.Message{},
	)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	return nil
}
