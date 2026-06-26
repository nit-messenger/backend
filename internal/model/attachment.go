package model

import (
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index"`
	FileName  string    `gorm:"size:255;not null"`
	FilePath  string    `gorm:"type:text;not null"`
	FileSize  int64     `gorm:"not null"`
	MimeType  string    `gorm:"size:128;not null"`
	CreatedAt time.Time
}
