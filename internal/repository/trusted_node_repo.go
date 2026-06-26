package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type trustedNodeRepo struct {
	db *gorm.DB
}

func NewTrustedNodeRepository(db *gorm.DB) TrustedNodeRepository {
	return &trustedNodeRepo{db: db}
}

func (r *trustedNodeRepo) Create(ctx context.Context, node *model.TrustedNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *trustedNodeRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.TrustedNode, error) {
	var node model.TrustedNode
	err := r.db.WithContext(ctx).First(&node, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *trustedNodeRepo) GetByDomain(ctx context.Context, domain string) (*model.TrustedNode, error) {
	var node model.TrustedNode
	err := r.db.WithContext(ctx).First(&node, "domain = ?", domain).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *trustedNodeRepo) GetByAPIKey(ctx context.Context, apiKey string) (*model.TrustedNode, error) {
	var node model.TrustedNode
	err := r.db.WithContext(ctx).First(&node, "api_key = ?", apiKey).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *trustedNodeRepo) List(ctx context.Context) ([]model.TrustedNode, error) {
	var nodes []model.TrustedNode
	err := r.db.WithContext(ctx).Order("domain ASC").Find(&nodes).Error
	return nodes, err
}

func (r *trustedNodeRepo) Update(ctx context.Context, node *model.TrustedNode) error {
	return r.db.WithContext(ctx).Save(node).Error
}

func (r *trustedNodeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.TrustedNode{}, "id = ?", id).Error
}
