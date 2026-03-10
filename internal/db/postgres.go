// Package db provides the PostgreSQL database connection and auto-migration.
package db

import (
	"fmt"
	"log"

	"github.com/samdevgo/go-chat-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a GORM connection to PostgreSQL using the provided DSN
// and automatically migrates all models to keep the schema up to date.
// It returns the open *gorm.DB instance or an error if the connection fails.
func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Use Info level logging so SQL queries are visible during development.
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("database: connected to PostgreSQL")

	// AutoMigrate creates or alters tables to match the current model structs.
	// It is safe to call on every startup — it never drops columns.
	if err := db.AutoMigrate(
		&models.User{},
		&models.Room{},
		&models.Message{},
	); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	log.Println("database: schema migration complete")
	return db, nil
}
