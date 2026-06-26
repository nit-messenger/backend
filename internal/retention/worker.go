package retention

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/corvych/nit/internal/model"
	"gorm.io/gorm"
)

type RetentionWorker struct {
	db       *gorm.DB
	interval time.Duration
}

func NewRetentionWorker(db *gorm.DB, interval time.Duration) *RetentionWorker {
	return &RetentionWorker{
		db:       db,
		interval: interval,
	}
}

func (w *RetentionWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	log.Println("Retention worker started")

	go func() {
		// Run once immediately on startup
		w.RunCleanup()

		for {
			select {
			case <-ticker.C:
				w.RunCleanup()
			case <-ctx.Done():
				ticker.Stop()
				log.Println("Retention worker stopped")
				return
			}
		}
	}()
}

func (w *RetentionWorker) RunCleanup() {
	log.Println("Running message retention cleanup...")

	// 1. Get default settings
	var settings model.ServerSettings
	if err := w.db.First(&settings).Error; err != nil {
		log.Printf("Retention Worker: failed to fetch server settings: %v", err)
		return
	}

	// 2. Fetch all conversations
	var conversations []model.Conversation
	if err := w.db.Find(&conversations).Error; err != nil {
		log.Printf("Retention Worker: failed to fetch conversations: %v", err)
		return
	}

	for _, conv := range conversations {
		// Determine retention days
		days := settings.DefaultRetentionDays
		if conv.RetentionDays != nil {
			days = conv.RetentionDays
		}

		if days == nil {
			continue // Keep forever
		}

		cutoff := time.Now().AddDate(0, 0, -*days)

		// 3. Find messages to delete (along with their attachments)
		var messages []model.Message
		err := w.db.Unscoped().
			Preload("Attachments").
			Where("conversation_id = ? AND created_at < ?", conv.ID, cutoff).
			Find(&messages).Error
		if err != nil {
			log.Printf("Retention Worker: failed to query expired messages in conversation %s: %v", conv.ID, err)
			continue
		}

		if len(messages) == 0 {
			continue
		}

		log.Printf("Retention Worker: cleaning up %d expired messages in conversation %s", len(messages), conv.ID)

		for _, msg := range messages {
			// Delete files from storage
			for _, att := range msg.Attachments {
				if err := os.Remove(att.FilePath); err != nil && !os.IsNotExist(err) {
					log.Printf("Retention Worker: failed to delete file %s: %v", att.FilePath, err)
				}
			}

			// Hard delete message and cascaded relationships (like attachments) from database
			if err := w.db.Unscoped().Delete(&msg).Error; err != nil {
				log.Printf("Retention Worker: failed to delete message %s: %v", msg.ID, err)
			}
		}
	}
}
