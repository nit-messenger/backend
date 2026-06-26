package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists       = errors.New("username or email already exists")
	ErrInvalidInvite    = errors.New("invalid family invite code")
	ErrInvalidCreds     = errors.New("invalid username or password")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrRegistrationOpen = errors.New("direct registration is closed; require invite code")
	ErrAlreadyActivated = errors.New("server is already activated")
)

type RegisterRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	InviteCode  string `json:"invite_code"` // Required if server registration is closed
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ActivateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	FamilyName  string `json:"family_name"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*TokenResponse, error)
	Login(ctx context.Context, req LoginRequest) (*TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	
	// Activation
	Activate(ctx context.Context, req ActivateRequest) (*TokenResponse, error)
	IsActivated(ctx context.Context) (bool, error)
}

type authService struct {
	userRepo   repository.UserRepository
	familyRepo repository.FamilyRepository
	tokenRepo  repository.TokenRepository
	cfg        *config.Config
}

func NewAuthService(
	ur repository.UserRepository,
	fr repository.FamilyRepository,
	tr repository.TokenRepository,
	cfg *config.Config,
) AuthService {
	return &authService{
		userRepo:   ur,
		familyRepo: fr,
		tokenRepo:  tr,
		cfg:        cfg,
	}
}

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*TokenResponse, error) {
	// 1. Check if user already exists
	existing, _ := s.userRepo.GetByUsername(ctx, req.Username)
	if existing != nil {
		return nil, ErrUserExists
	}
	if req.Email != "" {
		existingEmail, _ := s.userRepo.GetByEmail(ctx, req.Email)
		if existingEmail != nil {
			return nil, ErrUserExists
		}
	}

	// 2. Validate family connection if invite code is supplied or mandatory
	var family *model.Family
	var err error
	if req.InviteCode != "" {
		family, err = s.familyRepo.GetByInviteCode(ctx, req.InviteCode)
		if err != nil || family == nil {
			return nil, ErrInvalidInvite
		}
	}

	// 3. Hash password
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 4. Create user
	userID := uuid.New()
	user := &model.User{
		ID:           userID,
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		Email:        &req.Email,
		PasswordHash: string(hashedBytes),
		Status:       "offline",
		ServerDomain: s.cfg.ServerDomain,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 5. Add user to family if invite code was used
	if family != nil {
		member := &model.FamilyMember{
			FamilyID: family.ID,
			UserID:   userID,
			Role:     "member",
			JoinedAt: time.Now(),
		}
		if err := s.familyRepo.AddMember(ctx, member); err != nil {
			return nil, err
		}
	}

	// 6. Generate tokens
	return s.generateTokenPair(ctx, userID)
}

func (s *authService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil || user == nil {
		return nil, ErrInvalidCreds
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCreds
	}

	return s.generateTokenPair(ctx, user.ID)
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	// Parse/validate refresh token claims
	tokenClaims, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTRefreshSecret), nil
	})

	if err != nil || !tokenClaims.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := tokenClaims.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	subStr, err := claims.GetSubject()
	if err != nil {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(subStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check if token exists in DB (revocation/rotation check)
	hash := s.hashToken(refreshToken)
	storedToken, err := s.tokenRepo.GetByHash(ctx, hash)
	if err != nil || storedToken == nil {
		return nil, ErrInvalidToken
	}

	// Revoke old refresh token (rotation)
	if err := s.tokenRepo.DeleteByHash(ctx, hash); err != nil {
		return nil, err
	}

	// Generate new token pair
	return s.generateTokenPair(ctx, userID)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	hash := s.hashToken(refreshToken)
	return s.tokenRepo.DeleteByHash(ctx, hash)
}

func (s *authService) IsActivated(ctx context.Context) (bool, error) {
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *authService) Activate(ctx context.Context, req ActivateRequest) (*TokenResponse, error) {
	// 1. Check if already activated
	activated, err := s.IsActivated(ctx)
	if err != nil {
		return nil, err
	}
	if activated {
		return nil, ErrAlreadyActivated
	}

	// 2. Hash password
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 3. Create admin user
	userID := uuid.New()
	user := &model.User{
		ID:           userID,
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		Email:        &req.Email,
		PasswordHash: string(hashedBytes),
		Status:       "offline",
		ServerDomain: s.cfg.ServerDomain,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 4. Create primary family
	familyID := uuid.New()
	family := &model.Family{
		ID:         familyID,
		Name:       req.FamilyName,
		InviteCode: "ADM" + uuid.New().String()[:6],
		CreatedBy:  userID,
		CreatedAt:  time.Now(),
	}

	if err := s.familyRepo.Create(ctx, family); err != nil {
		return nil, err
	}

	// 5. Add user to family as administrator
	member := &model.FamilyMember{
		FamilyID: familyID,
		UserID:   userID,
		Role:     "admin",
		JoinedAt: time.Now(),
	}
	if err := s.familyRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	// 6. Generate tokens
	return s.generateTokenPair(ctx, userID)
}

// Private helper to generate and store tokens
func (s *authService) generateTokenPair(ctx context.Context, userID uuid.UUID) (*TokenResponse, error) {
	// 1. Access Token (15 mins duration)
	accessTokenClaims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(15 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessStr, err := accessToken.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, err
	}

	// 2. Refresh Token (30 days duration)
	expTime := time.Now().Add(30 * 24 * time.Hour)
	refreshTokenClaims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": expTime.Unix(),
		"iat": time.Now().Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshStr, err := refreshToken.SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return nil, err
	}

	// 3. Store refresh token in DB
	hash := s.hashToken(refreshStr)
	dbToken := &model.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expTime,
	}

	if err := s.tokenRepo.StoreRefreshToken(ctx, dbToken); err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresAt:    expTime,
	}, nil
}

func (s *authService) hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
