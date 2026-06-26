package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Username     string     `gorm:"size:32;uniqueIndex;not null"`
	DisplayName  string     `gorm:"size:64;not null"`
	Email        *string    `gorm:"size:255;uniqueIndex"`
	PasswordHash string     `gorm:"size:255;not null"`
	AvatarURL    *string    `gorm:"type:text"`
	Status       string     `gorm:"size:16;not null;default:'offline'"` // online/offline/away
	ServerDomain string     `gorm:"size:255;not null"`                  // this server's domain for federation
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
