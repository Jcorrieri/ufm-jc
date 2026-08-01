package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/models"
	"gorm.io/gorm"
)

func openTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenSQLite(databasePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	return db
}

func TestOpenSQLiteDoesNotMigrate(t *testing.T) {
	db := openTestDatabase(t)
	if db.Migrator().HasTable(&models.User{}) {
		t.Fatal("OpenSQLite() created the users table")
	}
}

func TestMigrateCreatesSchemaAndIsRepeatable(t *testing.T) {
	db := openTestDatabase(t)
	for run := 1; run <= 2; run++ {
		if err := Migrate(db); err != nil {
			t.Fatalf("Migrate() run %d error = %v", run, err)
		}
	}

	modelsWithTables := []any{
		&models.User{},
		&models.PasswordResetToken{},
		&models.Listing{},
		&models.Image{},
		&models.Order{},
		&models.Conversation{},
		&models.Message{},
	}
	for _, model := range modelsWithTables {
		if !db.Migrator().HasTable(model) {
			t.Errorf("Migrate() did not create table for %T", model)
		}
	}
	if db.Migrator().HasColumn(&models.Image{}, "Data") {
		t.Error("Migrate() created obsolete image BLOB column")
	}
	for _, field := range []string{"ObjectKey", "Status", "ChecksumSHA256", "UploadedByID"} {
		if !db.Migrator().HasColumn(&models.Image{}, field) {
			t.Errorf("Migrate() did not create images.%s", field)
		}
	}
}

func TestSeedIsSuccessfulAndIdempotent(t *testing.T) {
	db := openTestDatabase(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	for run := 1; run <= 2; run++ {
		if err := Seed(context.Background(), db); err != nil {
			t.Fatalf("Seed() run %d error = %v", run, err)
		}
	}

	var userCount int64
	var listingCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := db.Model(&models.Listing{}).Count(&listingCount).Error; err != nil {
		t.Fatalf("count listings: %v", err)
	}
	if userCount != 11 || listingCount != 88 {
		t.Fatalf("Seed() counts = (%d users, %d listings), want (11, 88)",
			userCount, listingCount)
	}
}

func TestSeedRollsBackWhenInsertFails(t *testing.T) {
	db := openTestDatabase(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	existingUser := models.User{
		Email: "test@ufl.edu", PasswordHash: "hash", FirstName: "Existing", LastName: "User",
	}
	if err := db.Create(&existingUser).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	if err := Seed(context.Background(), db); err == nil {
		t.Fatal("Seed() error = nil, want duplicate user error")
	}

	var userCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("users after failed Seed() = %d, want 1", userCount)
	}
}
