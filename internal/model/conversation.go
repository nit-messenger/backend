package model

import (
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	ID            uuid.UUID                 `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	FamilyID      uuid.UUID                 `gorm:"type:uuid;not null;index"`
	Family        Family                    `gorm:"foreignKey:FamilyID;constraint:OnDelete:CASCADE;"`
	Type          string                    `gorm:"size:16;not null;default:'direct'"` // direct / group
	Title         *string                   `gorm:"size:128"`
	AvatarURL     *string                   `gorm:"type:text"`
	CreatedBy     uuid.UUID                 `gorm:"type:uuid;not null"`
	Creator       User                      `gorm:"foreignKey:CreatedBy"`
	RetentionDays *int                      `gorm:"default:null"` // null = use server default
	Participants  []ConversationParticipant `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ConversationParticipant struct {
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	User           User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	Role           string    `gorm:"size:16;not null;default:'member'"` // admin / member
	Muted          bool      `gorm:"not null;default:false"`
	JoinedAt       time.Time `gorm:"not null;default:now()"`
}
