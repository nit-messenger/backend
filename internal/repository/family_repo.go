package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type familyRepo struct {
	db *gorm.DB
}

func NewFamilyRepository(db *gorm.DB) FamilyRepository {
	return &familyRepo{db: db}
}

func (r *familyRepo) Create(ctx context.Context, family *model.Family) error {
	return r.db.WithContext(ctx).Create(family).Error
}

func (r *familyRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Family, error) {
	var family model.Family
	if err := r.db.WithContext(ctx).Preload("Creator").First(&family, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &family, nil
}

func (r *familyRepo) GetByInviteCode(ctx context.Context, code string) (*model.Family, error) {
	var family model.Family
	if err := r.db.WithContext(ctx).First(&family, "invite_code = ?", code).Error; err != nil {
		return nil, err
	}
	return &family, nil
}

func (r *familyRepo) AddMember(ctx context.Context, member *model.FamilyMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *familyRepo) GetMembers(ctx context.Context, familyID uuid.UUID) ([]model.FamilyMember, error) {
	var members []model.FamilyMember
	if err := r.db.WithContext(ctx).Preload("User").Where("family_id = ?", familyID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *familyRepo) IsMember(ctx context.Context, familyID uuid.UUID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FamilyMember{}).
		Where("family_id = ? AND user_id = ?", familyID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *familyRepo) Update(ctx context.Context, family *model.Family) error {
	return r.db.WithContext(ctx).Save(family).Error
}
