package model

import (
	"time"

	"github.com/google/uuid"
)

type TrustedProxy struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name          string    `gorm:"size:128;not null"`
	URL           string    `gorm:"type:text;not null"` // e.g. https://proxy.example.com
	APIKey        string    `gorm:"size:255;not null"`  // shared secret key for registration auth
	Status        string    `gorm:"size:16;not null;default:'active'"` // active / disabled
	LastPingedAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
