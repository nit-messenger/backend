package model

import (
	"time"

	"github.com/google/uuid"
)

type TrustedNode struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Domain      string    `gorm:"size:255;uniqueIndex;not null"`       // e.g. "family-jones.home"
	BaseURL     string    `gorm:"type:text;not null"`                  // e.g. "https://family-jones.home:8080"
	APIKey      string    `gorm:"size:255;not null"`                   // shared secret for auth
	DisplayName string    `gorm:"size:64"`
	Status      string    `gorm:"size:16;not null;default:'pending'"` // pending/active/disabled
	LastSeenAt  *time.Time
	CreatedAt   time.Time
}
