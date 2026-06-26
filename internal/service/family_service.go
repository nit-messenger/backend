package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrNotFamilyAdmin = errors.New("only family admin can perform this action")
	ErrAlreadyMember  = errors.New("user is already a member of this family")
)

type CreateFamilyRequest struct {
	Name string `json:"name"`
}

type FamilyResponse struct {
	ID         uuid.UUID            `json:"id"`
	Name       string               `json:"name"`
	InviteCode string               `json:"invite_code"`
	CreatorID  uuid.UUID            `json:"creator_id"`
	Members    []FamilyMemberDetail `json:"members,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
}

type FamilyMemberDetail struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

type FamilyService interface {
	CreateFamily(ctx context.Context, userID uuid.UUID, name string) (*FamilyResponse, error)
	GetFamilyDetails(ctx context.Context, userID uuid.UUID, familyID uuid.UUID) (*FamilyResponse, error)
	GenerateInviteCode(ctx context.Context, userID uuid.UUID, familyID uuid.UUID) (string, error)
	JoinFamily(ctx context.Context, userID uuid.UUID, inviteCode string) (*FamilyResponse, error)
}

type familyService struct {
	familyRepo repository.FamilyRepository
}

func NewFamilyService(fr repository.FamilyRepository) FamilyService {
	return &familyService{familyRepo: fr}
}

func (s *familyService) CreateFamily(ctx context.Context, userID uuid.UUID, name string) (*FamilyResponse, error) {
	inviteCode := s.generateRandomCode()

	family := &model.Family{
		ID:         uuid.New(),
		Name:       name,
		InviteCode: inviteCode,
		CreatedBy:  userID,
		CreatedAt:  time.Now(),
	}

	if err := s.familyRepo.Create(ctx, family); err != nil {
		return nil, err
	}

	// Add creator as admin
	member := &model.FamilyMember{
		FamilyID: family.ID,
		UserID:   userID,
		Role:     "admin",
		JoinedAt: time.Now(),
	}

	if err := s.familyRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return &FamilyResponse{
		ID:         family.ID,
		Name:       family.Name,
		InviteCode: family.InviteCode,
		CreatorID:  family.CreatedBy,
		CreatedAt:  family.CreatedAt,
	}, nil
}

func (s *familyService) GetFamilyDetails(ctx context.Context, userID uuid.UUID, familyID uuid.UUID) (*FamilyResponse, error) {
	// Verify user is member
	isMember, err := s.familyRepo.IsMember(ctx, familyID, userID)
	if err != nil || !isMember {
		return nil, errors.New("unauthorized family access")
	}

	family, err := s.familyRepo.GetByID(ctx, familyID)
	if err != nil {
		return nil, err
	}

	members, err := s.familyRepo.GetMembers(ctx, familyID)
	if err != nil {
		return nil, err
	}

	memberDetails := make([]FamilyMemberDetail, len(members))
	for i, m := range members {
		memberDetails[i] = FamilyMemberDetail{
			UserID:      m.UserID,
			Username:    m.User.Username,
			DisplayName: m.User.DisplayName,
			Role:        m.Role,
			JoinedAt:    m.JoinedAt,
		}
	}

	return &FamilyResponse{
		ID:         family.ID,
		Name:       family.Name,
		InviteCode: family.InviteCode,
		CreatorID:  family.CreatedBy,
		Members:    memberDetails,
		CreatedAt:  family.CreatedAt,
	}, nil
}

func (s *familyService) GenerateInviteCode(ctx context.Context, userID uuid.UUID, familyID uuid.UUID) (string, error) {
	// Verify user is admin of the family
	members, err := s.familyRepo.GetMembers(ctx, familyID)
	if err != nil {
		return "", err
	}

	isAdmin := false
	for _, m := range members {
		if m.UserID == userID && m.Role == "admin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		return "", ErrNotFamilyAdmin
	}

	family, err := s.familyRepo.GetByID(ctx, familyID)
	if err != nil {
		return "", err
	}

	newCode := s.generateRandomCode()
	family.InviteCode = newCode

	if err := s.familyRepo.Update(ctx, family); err != nil {
		return "", err
	}

	return newCode, nil
}

func (s *familyService) JoinFamily(ctx context.Context, userID uuid.UUID, inviteCode string) (*FamilyResponse, error) {
	family, err := s.familyRepo.GetByInviteCode(ctx, inviteCode)
	if err != nil || family == nil {
		return nil, ErrInvalidInvite
	}

	isMember, err := s.familyRepo.IsMember(ctx, family.ID, userID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrAlreadyMember
	}

	member := &model.FamilyMember{
		FamilyID: family.ID,
		UserID:   userID,
		Role:     "member",
		JoinedAt: time.Now(),
	}

	if err := s.familyRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return s.GetFamilyDetails(ctx, userID, family.ID)
}

func (s *familyService) generateRandomCode() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "12345678" // fallback
	}
	return hex.EncodeToString(b) // 8 characters
}
