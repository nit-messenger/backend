package service

import (
	"context"
	"errors"
	"testing"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
)

type MockProxyRepo struct {
	proxies map[uuid.UUID]*model.TrustedProxy
}

func (m *MockProxyRepo) Create(ctx context.Context, proxy *model.TrustedProxy) error {
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *MockProxyRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.TrustedProxy, error) {
	proxy, exists := m.proxies[id]
	if !exists {
		return nil, errors.New("proxy not found")
	}
	return proxy, nil
}

func (m *MockProxyRepo) List(ctx context.Context) ([]model.TrustedProxy, error) {
	var list []model.TrustedProxy
	for _, p := range m.proxies {
		list = append(list, *p)
	}
	return list, nil
}

func (m *MockProxyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.proxies, id)
	return nil
}

func (m *MockProxyRepo) Update(ctx context.Context, proxy *model.TrustedProxy) error {
	m.proxies[proxy.ID] = proxy
	return nil
}

func TestProxyService_Lifecycle(t *testing.T) {
	repo := &MockProxyRepo{
		proxies: make(map[uuid.UUID]*model.TrustedProxy),
	}
	cfg := &config.Config{
		ServerDomain: "localhost",
		ServerPort:   "8080",
	}

	service := NewProxyService(repo, cfg)
	ctx := context.Background()

	// 1. Add Proxy
	req := AddProxyRequest{
		Name:   "Stable Proxy Node",
		URL:    "https://proxy.example.com",
		APIKey: "super-secret-key",
	}

	res, err := service.AddProxy(ctx, req)
	if err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	if res.Name != "Stable Proxy Node" {
		t.Errorf("expected proxy name Stable Proxy Node, got %s", res.Name)
	}
	if res.Status != "active" {
		t.Errorf("expected status active, got %s", res.Status)
	}

	proxyID := res.ID

	// 2. List Active Proxies
	activeList, err := service.ListActiveProxies(ctx)
	if err != nil {
		t.Fatalf("failed to list active proxies: %v", err)
	}

	if len(activeList) != 1 {
		t.Errorf("expected 1 active proxy, got %d", len(activeList))
	}
	if activeList[0].URL != "https://proxy.example.com" {
		t.Errorf("expected proxy URL https://proxy.example.com, got %s", activeList[0].URL)
	}

	// 3. Update Status (disable it)
	err = service.UpdateProxyStatus(ctx, proxyID, "disabled")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Verify status updated
	allProxies, err := service.ListAllProxies(ctx)
	if err != nil {
		t.Fatalf("failed to list all proxies: %v", err)
	}

	if allProxies[0].Status != "disabled" {
		t.Errorf("expected proxy status disabled, got %s", allProxies[0].Status)
	}

	// Active list should be empty
	activeList, err = service.ListActiveProxies(ctx)
	if err != nil {
		t.Fatalf("failed to list active proxies: %v", err)
	}
	if len(activeList) != 0 {
		t.Errorf("expected 0 active proxies, got %d", len(activeList))
	}

	// 4. Delete Proxy
	err = service.DeleteProxy(ctx, proxyID)
	if err != nil {
		t.Fatalf("failed to delete proxy: %v", err)
	}

	allProxies, err = service.ListAllProxies(ctx)
	if err != nil {
		t.Fatalf("failed to list all proxies: %v", err)
	}
	if len(allProxies) != 0 {
		t.Errorf("expected 0 proxies, got %d", len(allProxies))
	}
}
