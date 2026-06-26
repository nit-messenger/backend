package database

import (
	"fmt"
	"log"
	"time"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)

	// Configure GORM logger (can be configured based on env, verbose for development)
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB
	var err error

	// Retry connection in case DB is starting up in docker-compose
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database. Retrying in 3 seconds... (%d/10)", i+1)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatalf("Fatal error connecting to database: %v", err)
	}

	log.Println("Database connection established")

	// Set connection pool limits
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to configure database pool: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Enable pgcrypto extension for UUID generation
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		log.Fatalf("Failed to enable pgcrypto extension: %v", err)
	}

	// AutoMigrate all tables
	err = db.AutoMigrate(
		&model.User{},
		&model.Family{},
		&model.FamilyMember{},
		&model.Conversation{},
		&model.ConversationParticipant{},
		&model.Message{},
		&model.Attachment{},
		&model.Call{},
		&model.CallParticipant{},
		&model.ServerSettings{},
		&model.MessageRead{},
		&model.PushSubscription{},
		&model.RefreshToken{},
		&model.TrustedNode{},
		&model.TrustedProxy{},
	)
	if err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration completed successfully")

	// Seed default ServerSettings if table is empty
	var count int64
	db.Model(&model.ServerSettings{}).Count(&count)
	if count == 0 {
		defaultSettings := model.ServerSettings{
			ID:                   1,
			ServerDomain:         cfg.ServerDomain,
			DefaultRetentionDays: nil, // null = forever
			MaxUploadBytes:       cfg.MaxUploadBytes,
			MediaStoragePath:     cfg.MediaStoragePath,
			RegistrationOpen:     false, // invite-only by default
		}
		if err := db.Create(&defaultSettings).Error; err != nil {
			log.Printf("Warning: Failed to seed default server settings: %v", err)
		} else {
			log.Println("Seeded default server settings")
		}
	}

	return db
}
