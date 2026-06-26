package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

type TrustedNodeResponse struct {
	ID          uuid.UUID  `json:"id"`
	Domain      string     `json:"domain"`
	BaseURL     string     `json:"base_url"`
	DisplayName string     `json:"display_name"`
	Status      string     `json:"status"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type AddNodeRequest struct {
	Domain      string `json:"domain"`
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key"`
	DisplayName string `json:"display_name"`
}

type FederationService interface {
	AddNode(ctx context.Context, req AddNodeRequest) (*TrustedNodeResponse, error)
	ListNodes(ctx context.Context) ([]TrustedNodeResponse, error)
	DeleteNode(ctx context.Context, id uuid.UUID) error
	UpdateNodeStatus(ctx context.Context, id uuid.UUID, status string) error
	
	// Outgoing Relay triggers
	RelayMessage(ctx context.Context, conversationID uuid.UUID, senderName string, content string, msgType string)
	RelayCallSignal(ctx context.Context, conversationID uuid.UUID, signalType string, payload interface{})
}

type federationService struct {
	nodeRepo repository.TrustedNodeRepository
	client   *http.Client
}

func NewFederationService(nr repository.TrustedNodeRepository) FederationService {
	return &federationService{
		nodeRepo: nr,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *federationService) AddNode(ctx context.Context, req AddNodeRequest) (*TrustedNodeResponse, error) {
	node := &model.TrustedNode{
		ID:          uuid.New(),
		Domain:      req.Domain,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		DisplayName: req.DisplayName,
		Status:      "active",
		CreatedAt:   time.Now(),
	}

	if err := s.nodeRepo.Create(ctx, node); err != nil {
		return nil, err
	}

	return s.mapNodeToResponse(node), nil
}

func (s *federationService) ListNodes(ctx context.Context) ([]TrustedNodeResponse, error) {
	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]TrustedNodeResponse, len(nodes))
	for i, n := range nodes {
		res[i] = *s.mapNodeToResponse(&n)
	}
	return res, nil
}

func (s *federationService) DeleteNode(ctx context.Context, id uuid.UUID) error {
	return s.nodeRepo.Delete(ctx, id)
}

func (s *federationService) UpdateNodeStatus(ctx context.Context, id uuid.UUID, status string) error {
	node, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	node.Status = status
	return s.nodeRepo.Update(ctx, node)
}

func (s *federationService) RelayMessage(ctx context.Context, conversationID uuid.UUID, senderName string, content string, msgType string) {
	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return
	}

	payload := map[string]interface{}{
		"conversation_id": conversationID.String(),
		"sender_name":     senderName,
		"content":         content,
		"type":            msgType,
	}

	for _, node := range nodes {
		if node.Status != "active" {
			continue
		}
		go s.sendHTTPRelay(node, "/api/federation/messages", payload)
	}
}

func (s *federationService) RelayCallSignal(ctx context.Context, conversationID uuid.UUID, signalType string, payload interface{}) {
	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return
	}

	body := map[string]interface{}{
		"conversation_id": conversationID.String(),
		"type":            signalType,
		"payload":         payload,
	}

	for _, node := range nodes {
		if node.Status != "active" {
			continue
		}
		go s.sendHTTPRelay(node, "/api/federation/calls", body)
	}
}

func (s *federationService) sendHTTPRelay(node model.TrustedNode, endpoint string, payload interface{}) {
	url := fmt.Sprintf("%s%s", node.BaseURL, endpoint)

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Federation Relay: failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Printf("Federation Relay: failed to create HTTP request to %s: %v", url, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Federation-Key", node.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("Federation Relay: failed to send HTTP request to %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("Federation Relay: remote node %s returned status: %d", url, resp.StatusCode)
		return
	}

	// Update LastSeenAt field in GORM
	now := time.Now()
	node.LastSeenAt = &now
	_ = s.nodeRepo.Update(context.Background(), &node)
}

func (s *federationService) mapNodeToResponse(node *model.TrustedNode) *TrustedNodeResponse {
	return &TrustedNodeResponse{
		ID:          node.ID,
		Domain:      node.Domain,
		BaseURL:     node.BaseURL,
		DisplayName: node.DisplayName,
		Status:      node.Status,
		LastSeenAt:  node.LastSeenAt,
		CreatedAt:   node.CreatedAt,
	}
}
