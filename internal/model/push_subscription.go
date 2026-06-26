package model

import (
	"time"

	"github.com/google/uuid"
)

type PushSubscription struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	Platform  string    `gorm:"size:16;not null"` // web / android / ios
	Token     string    `gorm:"type:text;not null"`
	Endpoint  *string   `gorm:"type:text"` // web push endpoint
	P256dh    *string   `gorm:"type:text"` // web push key
	Auth      *string   `gorm:"type:text"` // web push auth
	CreatedAt time.Time
}
