package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type callRepo struct {
	db *gorm.DB
}

func NewCallRepository(db *gorm.DB) CallRepository {
	return &callRepo{db: db}
}

func (r *callRepo) Create(ctx context.Context, call *model.Call) error {
	return r.db.WithContext(ctx).Create(call).Error
}

func (r *callRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Call, error) {
	var call model.Call
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Preload("Participants.User").
		First(&call, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &call, nil
}

func (r *callRepo) AddParticipant(ctx context.Context, participant *model.CallParticipant) error {
	return r.db.WithContext(ctx).Create(participant).Error
}

func (r *callRepo) UpdateParticipant(ctx context.Context, participant *model.CallParticipant) error {
	return r.db.WithContext(ctx).Save(participant).Error
}

func (r *callRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Call, error) {
	var calls []model.Call
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Preload("Participants.User").
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Find(&calls).Error
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (r *callRepo) Update(ctx context.Context, call *model.Call) error {
	return r.db.WithContext(ctx).Save(call).Error
}
