package model

import (
	"time"

	"github.com/google/uuid"
)

type Family struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name       string         `gorm:"size:64;not null"`
	InviteCode string         `gorm:"size:16;uniqueIndex;not null"`
	CreatedBy  uuid.UUID      `gorm:"type:uuid;not null"`
	Creator    User           `gorm:"foreignKey:CreatedBy"`
	Members    []FamilyMember `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt  time.Time
}

type FamilyMember struct {
	FamilyID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	Role      string    `gorm:"size:16;not null;default:'member'"` // admin / member
	JoinedAt  time.Time `gorm:"not null;default:now()"`
}
