package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/google/uuid"
)

type SubscribeRequest struct {
	Platform string  `json:"platform"` // web / android / ios
	Token    string  `json:"token"`    // push token or device identifier
	Endpoint *string `json:"endpoint,omitempty"`
	P256dh   *string `json:"p256dh,omitempty"`
	Auth     *string `json:"auth,omitempty"`
}

type PushService interface {
	Subscribe(ctx context.Context, userID uuid.UUID, req SubscribeRequest) error
	Unsubscribe(ctx context.Context, userID uuid.UUID, token string) error
	SendPushToUsers(ctx context.Context, userIDs []uuid.UUID, title, body string, data map[string]interface{}) error
}

type pushService struct {
	repo repository.PushSubscriptionRepository
	cfg  *config.Config
}

func NewPushService(repo repository.PushSubscriptionRepository, cfg *config.Config) PushService {
	return &pushService{
		repo: repo,
		cfg:  cfg,
	}
}

func (s *pushService) Subscribe(ctx context.Context, userID uuid.UUID, req SubscribeRequest) error {
	sub := &model.PushSubscription{
		ID:        uuid.New(),
		UserID:    userID,
		Platform:  req.Platform,
		Token:     req.Token,
		Endpoint:  req.Endpoint,
		P256dh:    req.P256dh,
		Auth:      req.Auth,
		CreatedAt: time.Now(),
	}

	return s.repo.Create(ctx, sub)
}

func (s *pushService) Unsubscribe(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteByToken(ctx, userID, token)
}

func (s *pushService) SendPushToUsers(ctx context.Context, userIDs []uuid.UUID, title, body string, data map[string]interface{}) error {
	subs, err := s.repo.ListByUserIDs(ctx, userIDs)
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		return nil
	}

	// Prepare payload
	payloadMap := map[string]interface{}{
		"notification": map[string]string{
			"title": title,
			"body":  body,
		},
		"data": data,
	}

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		switch sub.Platform {
		case "web":
			if s.cfg.VAPIDPublicKey == "" || s.cfg.VAPIDPrivateKey == "" {
				log.Println("PushService: VAPID keys not configured, skipping web push")
				continue
			}

			// Parse subscription keys
			p256dhVal := ""
			if sub.P256dh != nil {
				p256dhVal = *sub.P256dh
			}
			authVal := ""
			if sub.Auth != nil {
				authVal = *sub.Auth
			}
			endpointVal := ""
			if sub.Endpoint != nil {
				endpointVal = *sub.Endpoint
			}

			sSub := &webpush.Subscription{
				Endpoint: endpointVal,
				Keys: webpush.Keys{
					P256dh: p256dhVal,
					Auth:   authVal,
				},
			}

			// Send notification
			go func(sub model.PushSubscription, sSub *webpush.Subscription) {
				resp, err := webpush.SendNotification(payloadBytes, sSub, &webpush.Options{
					Subscriber:      "mailto:admin@" + s.cfg.ServerDomain,
					VAPIDPublicKey:  s.cfg.VAPIDPublicKey,
					VAPIDPrivateKey: s.cfg.VAPIDPrivateKey,
					TTL:             30,
				})
				if err != nil {
					log.Printf("PushService: failed to send web push to user %s: %v", sub.UserID, err)
					return
				}
				defer resp.Body.Close()
				log.Printf("PushService: sent web push to user %s (status: %d)", sub.UserID, resp.StatusCode)
			}(sub, sSub)

		case "android":
			// Android (FCM) mock delivery
			log.Printf("PushService [MOCK FCM]: Sending push to Android device (User: %s, Token: %s, Title: %s, Body: %s)",
				sub.UserID, sub.Token, title, body)

		case "ios":
			// iOS (APNs) mock delivery
			log.Printf("PushService [MOCK APNs]: Sending push to iOS device (User: %s, Token: %s, Title: %s, Body: %s)",
				sub.UserID, sub.Token, title, body)
		}
	}

	return nil
}
