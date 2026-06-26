package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerSettings struct {
	ID                   uint      `gorm:"primaryKey;default:1"`
	ServerDomain         string    `gorm:"size:255;not null"`
	DefaultRetentionDays *int      `gorm:"default:null"`                // null = keep forever
	MaxUploadBytes       int64     `gorm:"not null;default:2147483648"` // 2 GB
	MediaStoragePath     string    `gorm:"type:text;not null;default:'./uploads'"`
	RegistrationOpen     bool      `gorm:"not null;default:false"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type MessageRead struct {
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LastReadAt     time.Time `gorm:"not null;default:now()"`
}
