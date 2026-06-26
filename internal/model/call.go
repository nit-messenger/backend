package model

import (
	"time"

	"github.com/google/uuid"
)

type Call struct {
	ID             uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ConversationID uuid.UUID         `gorm:"type:uuid;not null;index"`
	Conversation   Conversation      `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE;"`
	InitiatedBy    uuid.UUID         `gorm:"type:uuid;not null"`
	Initiator      User              `gorm:"foreignKey:InitiatedBy"`
	Type           string            `gorm:"size:16;not null"` // audio / video
	Status         string            `gorm:"size:16;not null;default:'ringing'"` // ringing/active/ended/missed
	Participants   []CallParticipant `gorm:"constraint:OnDelete:CASCADE;"`
	StartedAt      *time.Time
	EndedAt        *time.Time
	CreatedAt      time.Time
}

type CallParticipant struct {
	CallID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	User     User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	JoinedAt *time.Time
	LeftAt   *time.Time
}
// Note: We'll also define MessageRead, PushSubscription, RefreshToken, TrustedNode, ServerSettings in respective files or group them.
