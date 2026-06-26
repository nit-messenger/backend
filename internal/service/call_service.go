package service

import (
	"context"
	"time"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/livekit"
	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/corvych/nit/internal/ws"
	"github.com/google/uuid"
)

type StartCallRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Type           string    `json:"type"` // audio / video
}

type CallResponse struct {
	ID             uuid.UUID                 `json:"id"`
	ConversationID uuid.UUID                 `json:"conversation_id"`
	CreatedBy      uuid.UUID                 `json:"created_by"`
	Type           string                    `json:"type"`
	Status         string                    `json:"status"` // active / ended
	CreatedAt      time.Time                 `json:"created_at"`
	EndedAt        *time.Time                `json:"ended_at,omitempty"`
	Participants   []CallParticipantResponse `json:"participants"`
	LiveKitURL     string                    `json:"livekit_url,omitempty"`
	LiveKitToken   string                    `json:"livekit_token,omitempty"`
	RoomName       string                    `json:"room_name,omitempty"`
}

type CallParticipantResponse struct {
	UserID      uuid.UUID  `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	JoinedAt    *time.Time `json:"joined_at,omitempty"`
	LeftAt      *time.Time `json:"left_at,omitempty"`
}

type CallService interface {
	StartCall(ctx context.Context, userID uuid.UUID, req StartCallRequest) (*CallResponse, error)
	JoinCall(ctx context.Context, userID uuid.UUID, callID uuid.UUID) (*CallResponse, error)
	LeaveCall(ctx context.Context, userID uuid.UUID, callID uuid.UUID) error
	GetCallHistory(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) ([]CallResponse, error)
}

type callService struct {
	callRepo         repository.CallRepository
	conversationRepo repository.ConversationRepository
	hub              *ws.Hub
	config           *config.Config
}

func NewCallService(cr repository.CallRepository, convRepo repository.ConversationRepository, hub *ws.Hub, cfg *config.Config) CallService {
	return &callService{
		callRepo:         cr,
		conversationRepo: convRepo,
		hub:              hub,
		config:           cfg,
	}
}

func (s *callService) StartCall(ctx context.Context, userID uuid.UUID, req StartCallRequest) (*CallResponse, error) {
	// Verify conversation participant
	isPart, err := s.conversationRepo.IsParticipant(ctx, req.ConversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	now := time.Now()
	call := &model.Call{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		InitiatedBy:    userID,
		Type:           req.Type,
		Status:         "active",
		StartedAt:      &now,
		CreatedAt:      now,
	}

	if err := s.callRepo.Create(ctx, call); err != nil {
		return nil, err
	}

	// Add creator as participant
	part := &model.CallParticipant{
		CallID:   call.ID,
		UserID:   userID,
		JoinedAt: &now,
	}
	_ = s.callRepo.AddParticipant(ctx, part)

	res, err := s.getCallResponse(ctx, call.ID)
	if err != nil {
		return nil, err
	}

	// Generate LiveKit Token
	roomName := res.ConversationID.String()
	identity := userID.String()
	token, err := livekit.GenerateToken(s.config.LiveKitAPIKey, s.config.LiveKitAPISecret, roomName, identity)
	if err == nil {
		res.LiveKitURL = s.config.LiveKitURL
		res.LiveKitToken = token
		res.RoomName = roomName
	}

	// Broadcast incoming call event to all participants
	conv, err := s.conversationRepo.GetByID(ctx, req.ConversationID)
	if err == nil && conv != nil {
		pIDs := make([]uuid.UUID, len(conv.Participants))
		for i, p := range conv.Participants {
			pIDs[i] = p.UserID
		}

		callerName := "Someone"
		for _, p := range res.Participants {
			if p.UserID == userID {
				if p.DisplayName != "" {
					callerName = p.DisplayName
				} else {
					callerName = p.Username
				}
				break
			}
		}

		s.hub.BroadcastToUsers("incoming_call", map[string]interface{}{
			"call_id":         res.ID,
			"conversation_id": res.ConversationID,
			"type":            res.Type,
			"caller_id":       userID,
			"caller_name":     callerName,
		}, pIDs)
	}

	return res, nil
}

func (s *callService) JoinCall(ctx context.Context, userID uuid.UUID, callID uuid.UUID) (*CallResponse, error) {
	call, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, err
	}

	// Verify conversation participant
	isPart, err := s.conversationRepo.IsParticipant(ctx, call.ConversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	// Check if already in call
	var existing *model.CallParticipant
	for _, p := range call.Participants {
		if p.UserID == userID && p.LeftAt == nil {
			existing = &p
			break
		}
	}

	if existing == nil {
		now := time.Now()
		part := &model.CallParticipant{
			CallID:   callID,
			UserID:   userID,
			JoinedAt: &now,
		}
		if err := s.callRepo.AddParticipant(ctx, part); err != nil {
			return nil, err
		}
	}

	res, err := s.getCallResponse(ctx, callID)
	if err != nil {
		return nil, err
	}

	// Generate LiveKit Token
	roomName := res.ConversationID.String()
	identity := userID.String()
	token, err := livekit.GenerateToken(s.config.LiveKitAPIKey, s.config.LiveKitAPISecret, roomName, identity)
	if err == nil {
		res.LiveKitURL = s.config.LiveKitURL
		res.LiveKitToken = token
		res.RoomName = roomName
	}

	return res, nil
}

func (s *callService) LeaveCall(ctx context.Context, userID uuid.UUID, callID uuid.UUID) error {
	call, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return err
	}

	// Find active participant record and set LeftAt
	for _, p := range call.Participants {
		if p.UserID == userID && p.LeftAt == nil {
			now := time.Now()
			p.LeftAt = &now
			_ = s.callRepo.UpdateParticipant(ctx, &p)
			break
		}
	}

	// Reload call to check if all participants have left
	updatedCall, err := s.callRepo.GetByID(ctx, callID)
	if err == nil {
		hasActive := false
		for _, p := range updatedCall.Participants {
			if p.LeftAt == nil {
				hasActive = true
				break
			}
		}

		// If no active participants remaining, end the call
		if !hasActive {
			now := time.Now()
			updatedCall.Status = "ended"
			updatedCall.EndedAt = &now
			_ = s.callRepo.Update(ctx, updatedCall)
		}
	}

	return nil
}

func (s *callService) GetCallHistory(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) ([]CallResponse, error) {
	// Verify conversation participant
	isPart, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	calls, err := s.callRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	responses := make([]CallResponse, len(calls))
	for i, c := range calls {
		parts := make([]CallParticipantResponse, len(c.Participants))
		for j, p := range c.Participants {
			parts[j] = CallParticipantResponse{
				UserID:      p.UserID,
				Username:    p.User.Username,
				DisplayName: p.User.DisplayName,
				JoinedAt:    p.JoinedAt,
				LeftAt:      p.LeftAt,
			}
		}
		responses[i] = CallResponse{
			ID:             c.ID,
			ConversationID: c.ConversationID,
			CreatedBy:      c.InitiatedBy,
			Type:           c.Type,
			Status:         c.Status,
			CreatedAt:      c.CreatedAt,
			EndedAt:        c.EndedAt,
			Participants:   parts,
		}
	}

	return responses, nil
}

func (s *callService) getCallResponse(ctx context.Context, callID uuid.UUID) (*CallResponse, error) {
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, err
	}

	parts := make([]CallParticipantResponse, len(c.Participants))
	for j, p := range c.Participants {
		parts[j] = CallParticipantResponse{
			UserID:      p.UserID,
			Username:    p.User.Username,
			DisplayName: p.User.DisplayName,
			JoinedAt:    p.JoinedAt,
			LeftAt:      p.LeftAt,
		}
	}

	return &CallResponse{
		ID:             c.ID,
		ConversationID: c.ConversationID,
		CreatedBy:      c.InitiatedBy,
		Type:           c.Type,
		Status:         c.Status,
		CreatedAt:      c.CreatedAt,
		EndedAt:        c.EndedAt,
		Participants:   parts,
	}, nil
}
