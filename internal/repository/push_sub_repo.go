package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type pushSubRepo struct {
	db *gorm.DB
}

func NewPushSubscriptionRepository(db *gorm.DB) PushSubscriptionRepository {
	return &pushSubRepo{db: db}
}

func (r *pushSubRepo) Create(ctx context.Context, sub *model.PushSubscription) error {
	// If subscription with this token/endpoint already exists for the user, update it or do nothing
	var existing model.PushSubscription
	err := r.db.WithContext(ctx).Where("user_id = ? AND token = ?", sub.UserID, sub.Token).First(&existing).Error
	if err == nil {
		sub.ID = existing.ID
		return r.db.WithContext(ctx).Save(sub).Error
	}
	return r.db.WithContext(ctx).Create(sub).Error
}

func (r *pushSubRepo) DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND token = ?", userID, token).Delete(&model.PushSubscription{}).Error
}

func (r *pushSubRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&subs).Error
	return subs, err
}

func (r *pushSubRepo) ListByUserIDs(ctx context.Context, userIDs []uuid.UUID) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	if len(userIDs) == 0 {
		return subs, nil
	}
	err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&subs).Error
	return subs, err
}
