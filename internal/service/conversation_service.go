package service

import (
	"context"
	"errors"
	"time"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrUnauthorizedAccess = errors.New("unauthorized conversation access")
	ErrConversationExists = errors.New("direct conversation already exists")
)

type CreateConversationRequest struct {
	FamilyID       uuid.UUID   `json:"family_id"`
	Type           string      `json:"type"` // direct / group
	Title          *string     `json:"title"`
	ParticipantIDs []uuid.UUID `json:"participant_ids"`
}

type ConversationResponse struct {
	ID            uuid.UUID            `json:"id"`
	FamilyID      uuid.UUID            `json:"family_id"`
	Type          string               `json:"type"`
	Title         *string              `json:"title"`
	AvatarURL     *string              `json:"avatar_url"`
	CreatedBy     uuid.UUID            `json:"created_by"`
	RetentionDays *int                 `json:"retention_days"`
	Participants  []ParticipantDetails `json:"participants"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type ParticipantDetails struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
}

type ConversationService interface {
	CreateConversation(ctx context.Context, userID uuid.UUID, req CreateConversationRequest) (*ConversationResponse, error)
	GetConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (*ConversationResponse, error)
	ListConversations(ctx context.Context, userID uuid.UUID) ([]ConversationResponse, error)
}

type conversationService struct {
	conversationRepo repository.ConversationRepository
	familyRepo       repository.FamilyRepository
}

func NewConversationService(cr repository.ConversationRepository, fr repository.FamilyRepository) ConversationService {
	return &conversationService{
		conversationRepo: cr,
		familyRepo:       fr,
	}
}

func (s *conversationService) CreateConversation(ctx context.Context, userID uuid.UUID, req CreateConversationRequest) (*ConversationResponse, error) {
	// 1. Verify caller is member of the family
	isMember, err := s.familyRepo.IsMember(ctx, req.FamilyID, userID)
	if err != nil || !isMember {
		return nil, ErrUnauthorizedAccess
	}

	// 2. Validate participants belong to family
	uniqueParticipants := make(map[uuid.UUID]bool)
	uniqueParticipants[userID] = true
	for _, pID := range req.ParticipantIDs {
		isPMember, err := s.familyRepo.IsMember(ctx, req.FamilyID, pID)
		if err == nil && isPMember {
			uniqueParticipants[pID] = true
		}
	}

	// Create list of unique participant IDs
	participantIDs := make([]uuid.UUID, 0, len(uniqueParticipants))
	for pID := range uniqueParticipants {
		participantIDs = append(participantIDs, pID)
	}

	// 3. For direct chats, enforce exactly 2 participants and check if one already exists
	if req.Type == "direct" {
		if len(participantIDs) != 2 {
			return nil, errors.New("direct chat requires exactly 2 participants")
		}

		// Look for existing direct conversation between these two
		existingConvList, err := s.conversationRepo.ListByUserID(ctx, userID)
		if err == nil {
			for _, c := range existingConvList {
				if c.Type == "direct" && c.FamilyID == req.FamilyID {
					pMap := make(map[uuid.UUID]bool)
					for _, p := range c.Participants {
						pMap[p.UserID] = true
					}
					if pMap[participantIDs[0]] && pMap[participantIDs[1]] {
						// Return details of existing conversation
						return s.GetConversation(ctx, userID, c.ID)
					}
				}
			}
		}
	}

	// 4. Create conversation
	convID := uuid.New()
	conversation := &model.Conversation{
		ID:        convID,
		FamilyID:  req.FamilyID,
		Type:      req.Type,
		Title:     req.Title,
		CreatedBy: userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, err
	}

	// 5. Add participants
	for _, pID := range participantIDs {
		role := "member"
		if pID == userID {
			role = "creator"
		}
		part := &model.ConversationParticipant{
			ConversationID: convID,
			UserID:         pID,
			Role:           role,
			Muted:          false,
			JoinedAt:       time.Now(),
		}
		_ = s.conversationRepo.AddParticipant(ctx, part)
	}

	return s.GetConversation(ctx, userID, convID)
}

func (s *conversationService) GetConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (*ConversationResponse, error) {
	// Verify participant
	isPart, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	conv, err := s.conversationRepo.GetByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	parts := make([]ParticipantDetails, len(conv.Participants))
	for i, p := range conv.Participants {
		parts[i] = ParticipantDetails{
			UserID:      p.UserID,
			Username:    p.User.Username,
			DisplayName: p.User.DisplayName,
			Role:        p.Role,
		}
	}

	return &ConversationResponse{
		ID:            conv.ID,
		FamilyID:      conv.FamilyID,
		Type:          conv.Type,
		Title:         conv.Title,
		AvatarURL:     conv.AvatarURL,
		CreatedBy:     conv.CreatedBy,
		RetentionDays: conv.RetentionDays,
		Participants:  parts,
		CreatedAt:     conv.CreatedAt,
		UpdatedAt:     conv.UpdatedAt,
	}, nil
}

func (s *conversationService) ListConversations(ctx context.Context, userID uuid.UUID) ([]ConversationResponse, error) {
	conversations, err := s.conversationRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]ConversationResponse, len(conversations))
	for i, conv := range conversations {
		parts := make([]ParticipantDetails, len(conv.Participants))
		for j, p := range conv.Participants {
			parts[j] = ParticipantDetails{
				UserID:      p.UserID,
				Username:    p.User.Username,
				DisplayName: p.User.DisplayName,
				Role:        p.Role,
			}
		}
		responses[i] = ConversationResponse{
			ID:            conv.ID,
			FamilyID:      conv.FamilyID,
			Type:          conv.Type,
			Title:         conv.Title,
			AvatarURL:     conv.AvatarURL,
			CreatedBy:     conv.CreatedBy,
			RetentionDays: conv.RetentionDays,
			Participants:  parts,
			CreatedAt:     conv.CreatedAt,
			UpdatedAt:     conv.UpdatedAt,
		}
	}

	return responses, nil
}
