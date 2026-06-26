package service

import (
	"context"
	"errors"
	"time"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/corvych/nit/internal/ws"
	"github.com/google/uuid"
)

var (
	ErrNotMessageSender = errors.New("only the sender can edit or delete this message")
)

type SendMessageRequest struct {
	ConversationID uuid.UUID  `json:"conversation_id"`
	Content        *string    `json:"content"`
	Type           string     `json:"type"` // text / image / file / voice
	ReplyToID      *uuid.UUID `json:"reply_to_id"`
}

type MessageResponse struct {
	ID             uuid.UUID            `json:"id"`
	ConversationID uuid.UUID            `json:"conversation_id"`
	SenderID       uuid.UUID            `json:"sender_id"`
	SenderName     string               `json:"sender_name"`
	ReplyToID      *uuid.UUID           `json:"reply_to_id"`
	Content        *string              `json:"content"`
	Type           string               `json:"type"`
	Attachments    []AttachmentResponse `json:"attachments,omitempty"`
	EditedAt       *time.Time           `json:"edited_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

type AttachmentResponse struct {
	ID       uuid.UUID `json:"id"`
	FileName string    `json:"file_name"`
	FileSize int64     `json:"file_size"`
	MimeType string    `json:"mime_type"`
}

type MessageService interface {
	SendMessage(ctx context.Context, userID uuid.UUID, req SendMessageRequest) (*MessageResponse, error)
	ListMessages(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, limit int, beforeID *uuid.UUID) ([]MessageResponse, error)
	EditMessage(ctx context.Context, userID uuid.UUID, messageID uuid.UUID, newContent string) (*MessageResponse, error)
	DeleteMessage(ctx context.Context, userID uuid.UUID, messageID uuid.UUID) error
	MarkConversationAsRead(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error
}

type messageService struct {
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	hub              *ws.Hub
	pushService      PushService
	fedService       FederationService
}

func NewMessageService(
	mr repository.MessageRepository,
	cr repository.ConversationRepository,
	hub *ws.Hub,
	ps PushService,
	fs FederationService,
) MessageService {
	return &messageService{
		messageRepo:      mr,
		conversationRepo: cr,
		hub:              hub,
		pushService:      ps,
		fedService:       fs,
	}
}

func (s *messageService) SendMessage(ctx context.Context, userID uuid.UUID, req SendMessageRequest) (*MessageResponse, error) {
	// 1. Verify user is participant in conversation
	isPart, err := s.conversationRepo.IsParticipant(ctx, req.ConversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	// 2. Create message
	message := &model.Message{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		SenderID:       userID,
		ReplyToID:      req.ReplyToID,
		Content:        req.Content,
		Type:           req.Type,
		CreatedAt:      time.Now(),
	}

	if err := s.messageRepo.Create(ctx, message); err != nil {
		return nil, err
	}

	// 3. Touch updated_at for conversation
	conv, err := s.conversationRepo.GetByID(ctx, req.ConversationID)
	if err == nil {
		conv.UpdatedAt = time.Now()
		_ = s.conversationRepo.Update(ctx, conv)
	}

	res, err := s.mapToResponse(ctx, message)
	if err != nil {
		return nil, err
	}

	// 4. Broadcast via WebSocket to all participants
	if conv != nil {
		pIDs := make([]uuid.UUID, len(conv.Participants))
		for i, p := range conv.Participants {
			pIDs[i] = p.UserID
		}
		s.hub.BroadcastToUsers("new_message", res, pIDs)
	}

	// 5. Send push notification to offline participants
	if conv != nil {
		var offlineUserIDs []uuid.UUID
		for _, p := range conv.Participants {
			if p.UserID != userID && !s.hub.IsUserOnline(p.UserID) {
				offlineUserIDs = append(offlineUserIDs, p.UserID)
			}
		}

		if len(offlineUserIDs) > 0 {
			contentStr := ""
			if req.Content != nil {
				contentStr = *req.Content
			} else {
				contentStr = "Sent an attachment (" + req.Type + ")"
			}

			// Send asynchronously
			go func() {
				_ = s.pushService.SendPushToUsers(context.Background(), offlineUserIDs, res.SenderName, contentStr, map[string]interface{}{
					"conversation_id": req.ConversationID.String(),
					"message_id":      res.ID.String(),
				})
			}()
		}
	}

	// 6. Relay message to remote trusted nodes (federation)
	contentStr := ""
	if req.Content != nil {
		contentStr = *req.Content
	}
	s.fedService.RelayMessage(context.Background(), req.ConversationID, res.SenderName, contentStr, req.Type)

	return res, nil
}

func (s *messageService) ListMessages(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, limit int, beforeID *uuid.UUID) ([]MessageResponse, error) {
	// Verify user is participant
	isPart, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil || !isPart {
		return nil, ErrUnauthorizedAccess
	}

	messages, err := s.messageRepo.ListByConversationID(ctx, conversationID, limit, beforeID)
	if err != nil {
		return nil, err
	}

	responses := make([]MessageResponse, len(messages))
	for i, msg := range messages {
		res, err := s.mapToResponse(ctx, &msg)
		if err == nil {
			responses[i] = *res
		}
	}

	return responses, nil
}

func (s *messageService) EditMessage(ctx context.Context, userID uuid.UUID, messageID uuid.UUID, newContent string) (*MessageResponse, error) {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}

	if msg.SenderID != userID {
		return nil, ErrNotMessageSender
	}

	msg.Content = &newContent
	now := time.Now()
	msg.EditedAt = &now

	if err := s.messageRepo.Update(ctx, msg); err != nil {
		return nil, err
	}

	res, err := s.mapToResponse(ctx, msg)
	if err != nil {
		return nil, err
	}

	// Broadcast update
	conv, err := s.conversationRepo.GetByID(ctx, msg.ConversationID)
	if err == nil && conv != nil {
		pIDs := make([]uuid.UUID, len(conv.Participants))
		for i, p := range conv.Participants {
			pIDs[i] = p.UserID
		}
		s.hub.BroadcastToUsers("edit_message", res, pIDs)
	}

	return res, nil
}

func (s *messageService) DeleteMessage(ctx context.Context, userID uuid.UUID, messageID uuid.UUID) error {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}

	if msg.SenderID != userID {
		return ErrNotMessageSender
	}

	if err := s.messageRepo.Delete(ctx, messageID); err != nil {
		return err
	}

	// Broadcast deletion
	conv, err := s.conversationRepo.GetByID(ctx, msg.ConversationID)
	if err == nil && conv != nil {
		pIDs := make([]uuid.UUID, len(conv.Participants))
		for i, p := range conv.Participants {
			pIDs[i] = p.UserID
		}
		s.hub.BroadcastToUsers("delete_message", map[string]string{
			"id":              messageID.String(),
			"conversation_id": msg.ConversationID.String(),
		}, pIDs)
	}

	return nil
}

func (s *messageService) MarkConversationAsRead(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error {
	isPart, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil || !isPart {
		return ErrUnauthorizedAccess
	}

	if err := s.messageRepo.MarkRead(ctx, userID, conversationID); err != nil {
		return err
	}

	// Broadcast read receipt
	conv, err := s.conversationRepo.GetByID(ctx, conversationID)
	if err == nil && conv != nil {
		pIDs := make([]uuid.UUID, len(conv.Participants))
		for i, p := range conv.Participants {
			pIDs[i] = p.UserID
		}

		s.hub.BroadcastToUsers("read", ws.ReadReceiptPayload{
			ConversationID: conversationID.String(),
			UserID:         userID.String(),
			LastReadAt:     time.Now().Format(time.RFC3339),
		}, pIDs)
	}

	return nil
}

// Map database model to API response
func (s *messageService) mapToResponse(ctx context.Context, msg *model.Message) (*MessageResponse, error) {
	senderName := "Unknown"
	if msg.Sender.DisplayName != "" {
		senderName = msg.Sender.DisplayName
	}

	attachResponses := make([]AttachmentResponse, len(msg.Attachments))
	for i, att := range msg.Attachments {
		attachResponses[i] = AttachmentResponse{
			ID:       att.ID,
			FileName: att.FileName,
			FileSize: att.FileSize,
			MimeType: att.MimeType,
		}
	}

	return &MessageResponse{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		SenderName:     senderName,
		ReplyToID:      msg.ReplyToID,
		Content:        msg.Content,
		Type:           msg.Type,
		Attachments:    attachResponses,
		EditedAt:       msg.EditedAt,
		CreatedAt:      msg.CreatedAt,
	}, nil
}
