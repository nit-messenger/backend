package repository

import (
	"context"
	"time"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type messageRepo struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepo{db: db}
}

func (r *messageRepo) Create(ctx context.Context, message *model.Message) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *messageRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var message model.Message
	err := r.db.WithContext(ctx).
		Preload("Sender").
		Preload("Attachments").
		First(&message, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *messageRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID, limit int, beforeID *uuid.UUID) ([]model.Message, error) {
	query := r.db.WithContext(ctx).
		Preload("Sender").
		Preload("Attachments").
		Where("conversation_id = ?", conversationID)

	if beforeID != nil {
		var beforeMsg model.Message
		if err := r.db.WithContext(ctx).First(&beforeMsg, "id = ?", beforeID).Error; err == nil {
			query = query.Where("created_at < ?", beforeMsg.CreatedAt)
		}
	}

	var messages []model.Message
	err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *messageRepo) Update(ctx context.Context, message *model.Message) error {
	return r.db.WithContext(ctx).Save(message).Error
}

func (r *messageRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Message{}, "id = ?", id).Error
}

func (r *messageRepo) MarkRead(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error {
	readReceipt := model.MessageRead{
		UserID:         userID,
		ConversationID: conversationID,
		LastReadAt:     time.Now(),
	}

	// Upsert read receipt
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "conversation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_read_at"}),
	}).Create(&readReceipt).Error
}
