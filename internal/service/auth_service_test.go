package service

import (
	"context"
	"errors"
	"testing"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/google/uuid"
)

// MockUserRepo implements repository.UserRepository
type MockUserRepo struct {
	users map[string]*model.User
}

func (m *MockUserRepo) Create(ctx context.Context, user *model.User) error {
	m.users[user.Username] = user
	return nil
}

func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	u, exists := m.users[username]
	if !exists {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	for _, u := range m.users {
		if u.Email != nil && *u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockUserRepo) Update(ctx context.Context, user *model.User) error {
	m.users[user.Username] = user
	return nil
}

func (m *MockUserRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.Status = status
			return nil
		}
	}
	return errors.New("not found")
}

func (m *MockUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

// MockFamilyRepo implements repository.FamilyRepository
type MockFamilyRepo struct {
	families map[uuid.UUID]*model.Family
	members  map[uuid.UUID][]model.FamilyMember
}

func (m *MockFamilyRepo) Create(ctx context.Context, family *model.Family) error {
	m.families[family.ID] = family
	return nil
}

func (m *MockFamilyRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Family, error) {
	f, ok := m.families[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return f, nil
}

func (m *MockFamilyRepo) GetByInviteCode(ctx context.Context, code string) (*model.Family, error) {
	for _, f := range m.families {
		if f.InviteCode == code {
			return f, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockFamilyRepo) AddMember(ctx context.Context, member *model.FamilyMember) error {
	m.members[member.FamilyID] = append(m.members[member.FamilyID], *member)
	return nil
}

func (m *MockFamilyRepo) GetMembers(ctx context.Context, familyID uuid.UUID) ([]model.FamilyMember, error) {
	return m.members[familyID], nil
}

func (m *MockFamilyRepo) IsMember(ctx context.Context, familyID uuid.UUID, userID uuid.UUID) (bool, error) {
	for _, m := range m.members[familyID] {
		if m.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockFamilyRepo) Update(ctx context.Context, family *model.Family) error {
	m.families[family.ID] = family
	return nil
}

// MockTokenRepo implements repository.TokenRepository
type MockTokenRepo struct {
	tokens map[string]*model.RefreshToken
}

func (m *MockTokenRepo) StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *MockTokenRepo) GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *MockTokenRepo) DeleteByHash(ctx context.Context, hash string) error {
	delete(m.tokens, hash)
	return nil
}

func (m *MockTokenRepo) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	for k, t := range m.tokens {
		if t.UserID == userID {
			delete(m.tokens, k)
		}
	}
	return nil
}

func TestAuthService_Activation(t *testing.T) {
	userRepo := &MockUserRepo{users: make(map[string]*model.User)}
	familyRepo := &MockFamilyRepo{
		families: make(map[uuid.UUID]*model.Family),
		members:  make(map[uuid.UUID][]model.FamilyMember),
	}
	tokenRepo := &MockTokenRepo{tokens: make(map[string]*model.RefreshToken)}
	
	cfg := &config.Config{
		JWTSecret:        "test-secret",
		JWTRefreshSecret: "test-refresh-secret",
		ServerDomain:     "testdomain.com",
	}

	authService := NewAuthService(userRepo, familyRepo, tokenRepo, cfg)
	ctx := context.Background()

	// 1. Initial State: should not be activated
	activated, err := authService.IsActivated(ctx)
	if err != nil {
		t.Fatalf("failed to check activation: %v", err)
	}
	if activated {
		t.Errorf("expected server to be unactivated initially")
	}

	// 2. Perform Activation
	req := ActivateRequest{
		Username:    "admin",
		DisplayName: "System Administrator",
		Email:       "admin@example.com",
		Password:    "SecurePass123",
		FamilyName:  "Admin Family Workspace",
	}

	tokenResp, err := authService.Activate(ctx, req)
	if err != nil {
		t.Fatalf("failed to activate server: %v", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		t.Errorf("expected valid tokens returned, got empty strings")
	}

	// 3. Post-Activation State: should be activated
	activated, err = authService.IsActivated(ctx)
	if err != nil {
		t.Fatalf("failed to check activation: %v", err)
	}
	if !activated {
		t.Errorf("expected server to be activated after calling Activate")
	}

	// Verify User & Family structures created
	adminUser, err := userRepo.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("admin user not found in repo: %v", err)
	}

	if adminUser.DisplayName != "System Administrator" {
		t.Errorf("expected display name 'System Administrator', got %s", adminUser.DisplayName)
	}

	if len(familyRepo.families) != 1 {
		t.Errorf("expected 1 family to be created, got %d", len(familyRepo.families))
	}

	var primaryFamily *model.Family
	for _, f := range familyRepo.families {
		primaryFamily = f
		break
	}

	if primaryFamily.Name != "Admin Family Workspace" {
		t.Errorf("expected family name 'Admin Family Workspace', got %s", primaryFamily.Name)
	}

	// Verify member created and role set as admin
	isMem, err := familyRepo.IsMember(ctx, primaryFamily.ID, adminUser.ID)
	if err != nil || !isMem {
		t.Errorf("expected admin user to be a member of primary family")
	}

	members, _ := familyRepo.GetMembers(ctx, primaryFamily.ID)
	if len(members) != 1 || members[0].Role != "admin" {
		t.Errorf("expected member role to be 'admin', got %+v", members)
	}

	// 4. Multiple Activations: should fail
	_, err = authService.Activate(ctx, req)
	if !errors.Is(err, ErrAlreadyActivated) {
		t.Errorf("expected ErrAlreadyActivated on second call, got %v", err)
	}
}
