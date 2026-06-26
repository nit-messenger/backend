package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TrustedProxyRepository interface {
	Create(ctx context.Context, proxy *model.TrustedProxy) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TrustedProxy, error)
	List(ctx context.Context) ([]model.TrustedProxy, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, proxy *model.TrustedProxy) error
}

type trustedProxyRepository struct {
	db *gorm.DB
}

func NewTrustedProxyRepository(db *gorm.DB) TrustedProxyRepository {
	return &trustedProxyRepository{db: db}
}

func (r *trustedProxyRepository) Create(ctx context.Context, proxy *model.TrustedProxy) error {
	return r.db.WithContext(ctx).Create(proxy).Error
}

func (r *trustedProxyRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TrustedProxy, error) {
	var proxy model.TrustedProxy
	if err := r.db.WithContext(ctx).First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (r *trustedProxyRepository) List(ctx context.Context) ([]model.TrustedProxy, error) {
	var proxies []model.TrustedProxy
	if err := r.db.WithContext(ctx).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

func (r *trustedProxyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.TrustedProxy{}, "id = ?", id).Error
}

func (r *trustedProxyRepository) Update(ctx context.Context, proxy *model.TrustedProxy) error {
	return r.db.WithContext(ctx).Save(proxy).Error
}
