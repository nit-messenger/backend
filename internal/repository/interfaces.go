package repository

import (
	"context"

	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	Count(ctx context.Context) (int64, error)
}

type FamilyRepository interface {
	Create(ctx context.Context, family *model.Family) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Family, error)
	GetByInviteCode(ctx context.Context, code string) (*model.Family, error)
	AddMember(ctx context.Context, member *model.FamilyMember) error
	GetMembers(ctx context.Context, familyID uuid.UUID) ([]model.FamilyMember, error)
	IsMember(ctx context.Context, familyID uuid.UUID, userID uuid.UUID) (bool, error)
	Update(ctx context.Context, family *model.Family) error
}

type TokenRepository interface {
	StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	DeleteByHash(ctx context.Context, hash string) error
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

type ConversationRepository interface {
	Create(ctx context.Context, conversation *model.Conversation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Conversation, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Conversation, error)
	AddParticipant(ctx context.Context, participant *model.ConversationParticipant) error
	IsParticipant(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error)
	Update(ctx context.Context, conversation *model.Conversation) error
}

type MessageRepository interface {
	Create(ctx context.Context, message *model.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID, limit int, beforeID *uuid.UUID) ([]model.Message, error)
	Update(ctx context.Context, message *model.Message) error
	Delete(ctx context.Context, id uuid.UUID) error
	MarkRead(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error
}

type CallRepository interface {
	Create(ctx context.Context, call *model.Call) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Call, error)
	AddParticipant(ctx context.Context, participant *model.CallParticipant) error
	UpdateParticipant(ctx context.Context, participant *model.CallParticipant) error
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Call, error)
	Update(ctx context.Context, call *model.Call) error
}

type PushSubscriptionRepository interface {
	Create(ctx context.Context, sub *model.PushSubscription) error
	DeleteByToken(ctx context.Context, userID uuid.UUID, token string) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.PushSubscription, error)
	ListByUserIDs(ctx context.Context, userIDs []uuid.UUID) ([]model.PushSubscription, error)
}

type TrustedNodeRepository interface {
	Create(ctx context.Context, node *model.TrustedNode) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TrustedNode, error)
	GetByDomain(ctx context.Context, domain string) (*model.TrustedNode, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*model.TrustedNode, error)
	List(ctx context.Context) ([]model.TrustedNode, error)
	Update(ctx context.Context, node *model.TrustedNode) error
	Delete(ctx context.Context, id uuid.UUID) error
}
