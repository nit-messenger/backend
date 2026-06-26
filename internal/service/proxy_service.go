package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

type TrustedProxyResponse struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Status       string     `json:"status"`
	LastPingedAt *time.Time `json:"last_pinged_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ClientProxyResponse struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type AddProxyRequest struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type ProxyService interface {
	AddProxy(ctx context.Context, req AddProxyRequest) (*TrustedProxyResponse, error)
	ListAllProxies(ctx context.Context) ([]TrustedProxyResponse, error)
	ListActiveProxies(ctx context.Context) ([]ClientProxyResponse, error)
	DeleteProxy(ctx context.Context, id uuid.UUID) error
	UpdateProxyStatus(ctx context.Context, id uuid.UUID, status string) error
	
	// Registration
	RegisterToAllProxies(ctx context.Context)
	StartRegistrationWorker(ctx context.Context)
}

type proxyService struct {
	proxyRepo repository.TrustedProxyRepository
	cfg       *config.Config
	client    *http.Client
}

func NewProxyService(pr repository.TrustedProxyRepository, cfg *config.Config) ProxyService {
	return &proxyService{
		proxyRepo: pr,
		cfg:       cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *proxyService) AddProxy(ctx context.Context, req AddProxyRequest) (*TrustedProxyResponse, error) {
	proxy := &model.TrustedProxy{
		ID:        uuid.New(),
		Name:      req.Name,
		URL:       req.URL,
		APIKey:    req.APIKey,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	if err := s.proxyRepo.Create(ctx, proxy); err != nil {
		return nil, err
	}

	return s.mapToResponse(proxy), nil
}

func (s *proxyService) ListAllProxies(ctx context.Context) ([]TrustedProxyResponse, error) {
	proxies, err := s.proxyRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]TrustedProxyResponse, len(proxies))
	for i, p := range proxies {
		res[i] = *s.mapToResponse(&p)
	}
	return res, nil
}

func (s *proxyService) ListActiveProxies(ctx context.Context) ([]ClientProxyResponse, error) {
	proxies, err := s.proxyRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	var res []ClientProxyResponse
	for _, p := range proxies {
		if p.Status == "active" {
			res = append(res, ClientProxyResponse{
				Name: p.Name,
				URL:  p.URL,
			})
		}
	}
	return res, nil
}

func (s *proxyService) DeleteProxy(ctx context.Context, id uuid.UUID) error {
	return s.proxyRepo.Delete(ctx, id)
}

func (s *proxyService) UpdateProxyStatus(ctx context.Context, id uuid.UUID, status string) error {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	proxy.Status = status
	return s.proxyRepo.Update(ctx, proxy)
}

func (s *proxyService) RegisterToAllProxies(ctx context.Context) {
	proxies, err := s.proxyRepo.List(ctx)
	if err != nil {
		return
	}

	// Resolve backend's own external target URL
	targetURL := os.Getenv("EXTERNAL_BACKEND_URL")
	if targetURL == "" {
		targetURL = fmt.Sprintf("http://%s:%s", s.cfg.ServerDomain, s.cfg.ServerPort)
	}

	payload := map[string]string{
		"target_url": targetURL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Proxy Service: failed to marshal registration payload: %v", err)
		return
	}

	for _, proxy := range proxies {
		if proxy.Status != "active" {
			continue
		}

		go func(p model.TrustedProxy) {
			url := fmt.Sprintf("%s/register-target", p.URL)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
			if err != nil {
				log.Printf("Proxy Service: failed to create registration request for %s: %v", p.Name, err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Proxy-Key", p.APIKey)

			resp, err := s.client.Do(req)
			if err != nil {
				log.Printf("Proxy Service: failed to ping proxy %s (%s): %v", p.Name, p.URL, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Proxy Service: proxy %s returned status %d", p.Name, resp.StatusCode)
				return
			}

			// Update last ping timestamp
			now := time.Now()
			p.LastPingedAt = &now
			_ = s.proxyRepo.Update(context.Background(), &p)
		}(proxy)
	}
}

func (s *proxyService) StartRegistrationWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		// Register immediately on start
		s.RegisterToAllProxies(ctx)

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.RegisterToAllProxies(ctx)
			}
		}
	}()
}

func (s *proxyService) mapToResponse(proxy *model.TrustedProxy) *TrustedProxyResponse {
	return &TrustedProxyResponse{
		ID:           proxy.ID,
		Name:         proxy.Name,
		URL:          proxy.URL,
		Status:       proxy.Status,
		LastPingedAt: proxy.LastPingedAt,
		CreatedAt:    proxy.CreatedAt,
	}
}
