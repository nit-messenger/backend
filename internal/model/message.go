package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index:idx_msg_conv_created,priority:1"`
	Conversation   Conversation   `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE;"`
	SenderID       uuid.UUID      `gorm:"type:uuid;not null;index"`
	Sender         User           `gorm:"foreignKey:SenderID"`
	ReplyToID      *uuid.UUID     `gorm:"type:uuid;index"`
	ReplyTo        *Message       `gorm:"foreignKey:ReplyToID;constraint:OnDelete:SET NULL;"`
	Content        *string        `gorm:"type:text"`
	Type           string         `gorm:"size:16;not null;default:'text'"` // text/image/file/voice/system
	Attachments    []Attachment   `gorm:"constraint:OnDelete:CASCADE;"`
	EditedAt       *time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"` // soft delete
	CreatedAt      time.Time      `gorm:"index:idx_msg_conv_created,priority:2,sort:desc"`
}
