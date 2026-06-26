package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type tokenRepo struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepo{db: db}
}

func (r *tokenRepo) StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *tokenRepo) GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	if err := r.db.WithContext(ctx).First(&token, "token_hash = ?", hash).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepo) DeleteByHash(ctx context.Context, hash string) error {
	return r.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&model.RefreshToken{}).Error
}

func (r *tokenRepo) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.RefreshToken{}).Error
}
