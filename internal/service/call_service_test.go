package service

import (
	"context"
	"errors"
	"testing"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/ws"
	"github.com/google/uuid"
)

// MockConversationRepo implements repository.ConversationRepository
type MockConversationRepo struct {
	participants map[string]bool
}

func (m *MockConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	return nil
}

func (m *MockConversationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Conversation, error) {
	return &model.Conversation{
		ID:        id,
		CreatedBy: uuid.New(),
		Participants: []model.ConversationParticipant{
			{UserID: uuid.New()},
		},
	}, nil
}

func (m *MockConversationRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Conversation, error) {
	return nil, nil
}

func (m *MockConversationRepo) AddParticipant(ctx context.Context, participant *model.ConversationParticipant) error {
	return nil
}

func (m *MockConversationRepo) IsParticipant(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error) {
	key := conversationID.String() + ":" + userID.String()
	return m.participants[key], nil
}

func (m *MockConversationRepo) Update(ctx context.Context, conversation *model.Conversation) error {
	return nil
}

// MockCallRepo implements repository.CallRepository
type MockCallRepo struct {
	calls        map[uuid.UUID]*model.Call
	participants map[uuid.UUID][]model.CallParticipant
}

func (m *MockCallRepo) Create(ctx context.Context, call *model.Call) error {
	m.calls[call.ID] = call
	return nil
}

func (m *MockCallRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Call, error) {
	call, exists := m.calls[id]
	if !exists {
		return nil, errors.New("call not found")
	}
	// Attach mocked participants
	call.Participants = m.participants[id]
	return call, nil
}

func (m *MockCallRepo) AddParticipant(ctx context.Context, participant *model.CallParticipant) error {
	m.participants[participant.CallID] = append(m.participants[participant.CallID], *participant)
	return nil
}

func (m *MockCallRepo) UpdateParticipant(ctx context.Context, participant *model.CallParticipant) error {
	list := m.participants[participant.CallID]
	for i, p := range list {
		if p.UserID == participant.UserID {
			list[i] = *participant
			break
		}
	}
	m.participants[participant.CallID] = list
	return nil
}

func (m *MockCallRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Call, error) {
	var list []model.Call
	for _, c := range m.calls {
		if c.ConversationID == conversationID {
			c.Participants = m.participants[c.ID]
			list = append(list, *c)
		}
	}
	return list, nil
}

func (m *MockCallRepo) Update(ctx context.Context, call *model.Call) error {
	m.calls[call.ID] = call
	return nil
}

func TestCallService_Lifecycle(t *testing.T) {
	convID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()

	convRepo := &MockConversationRepo{
		participants: map[string]bool{
			convID.String() + ":" + userID1.String(): true,
			convID.String() + ":" + userID2.String(): true,
		},
	}

	callRepo := &MockCallRepo{
		calls:        make(map[uuid.UUID]*model.Call),
		participants: make(map[uuid.UUID][]model.CallParticipant),
	}

	cfg := &config.Config{
		LiveKitURL:       "http://localhost:7880",
		LiveKitAPIKey:    "devkey",
		LiveKitAPISecret: "secret",
	}
	hub := ws.NewHub()
	callService := NewCallService(callRepo, convRepo, hub, cfg)
	ctx := context.Background()

	// 1. Start Call
	startReq := StartCallRequest{
		ConversationID: convID,
		Type:           "video",
	}

	res, err := callService.StartCall(ctx, userID1, startReq)
	if err != nil {
		t.Fatalf("failed to start call: %v", err)
	}

	if res.Status != "active" {
		t.Errorf("expected call status active, got %s", res.Status)
	}
	if res.Type != "video" {
		t.Errorf("expected call type video, got %s", res.Type)
	}
	if len(res.Participants) != 1 {
		t.Errorf("expected 1 participant, got %d", len(res.Participants))
	}

	callID := res.ID

	// 2. Join Call (user2 joins)
	res, err = callService.JoinCall(ctx, userID2, callID)
	if err != nil {
		t.Fatalf("failed to join call: %v", err)
	}

	if len(res.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(res.Participants))
	}

	// 3. Leave Call (user1 leaves)
	err = callService.LeaveCall(ctx, userID1, callID)
	if err != nil {
		t.Fatalf("failed to leave call: %v", err)
	}

	// Fetch history & check status (should still be active since user2 is in it)
	history, err := callService.GetCallHistory(ctx, userID1, convID)
	if err != nil {
		t.Fatalf("failed to get call history: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 call in history, got %d", len(history))
	}
	if history[0].Status != "active" {
		t.Errorf("expected call to be active since one user remains, got %s", history[0].Status)
	}

	// 4. Leave Call (user2 leaves)
	err = callService.LeaveCall(ctx, userID2, callID)
	if err != nil {
		t.Fatalf("failed to leave call: %v", err)
	}

	// Check history (should now be ended)
	history, err = callService.GetCallHistory(ctx, userID1, convID)
	if err != nil {
		t.Fatalf("failed to get call history: %v", err)
	}

	if history[0].Status != "ended" {
		t.Errorf("expected call to be ended after all left, got %s", history[0].Status)
	}
}
