package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type conversationRepo struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepo{db: db}
}

func (r *conversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

func (r *conversationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Conversation, error) {
	var conversation model.Conversation
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Preload("Participants.User").
		First(&conversation, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *conversationRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Conversation, error) {
	var conversations []model.Conversation
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Preload("Participants.User").
		Joins("JOIN conversation_participants cp ON cp.conversation_id = conversations.id").
		Where("cp.user_id = ?", userID).
		Order("conversations.updated_at DESC").
		Find(&conversations).Error
	if err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *conversationRepo) AddParticipant(ctx context.Context, participant *model.ConversationParticipant) error {
	return r.db.WithContext(ctx).Create(participant).Error
}

func (r *conversationRepo) IsParticipant(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *conversationRepo) Update(ctx context.Context, conversation *model.Conversation) error {
	return r.db.WithContext(ctx).Save(conversation).Error
}
