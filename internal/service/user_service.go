package service

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

type UserService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	Update(ctx context.Context, id uuid.UUID, displayName string, email string, avatarURL *string) (*model.User, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(ur repository.UserRepository) UserService {
	return &userService{userRepo: ur}
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *userService) Update(ctx context.Context, id uuid.UUID, displayName string, email string, avatarURL *string) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if displayName != "" {
		user.DisplayName = displayName
	}
	if email != "" {
		user.Email = &email
	}
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
