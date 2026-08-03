package db

import (
	"context"
	"log"
	"sync"
	"time"

	"form_pf_backend/config"
	"form_pf_backend/models"
)

type submissionCache struct {
	mu        sync.RWMutex
	data      []models.FormDataLegacy
	updatedAt time.Time
}

var (
	submissionChangeCache *submissionCache
	stopRefresh           chan struct{}
)

// InitSubmissionCache starts the background refresher.
func InitSubmissionCache(interval time.Duration) {
	submissionChangeCache = &submissionCache{
		data: make([]models.FormDataLegacy, 0),
	}

	// Initial load with NO timeout — let it take whatever it needs on startup
	if err := refreshSubmissionCacheWithTimeout(0); err != nil {
		log.Printf("submission cache: initial load failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	stopRefresh = make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				// Background refreshes get 30s
				if err := refreshSubmissionCacheWithTimeout(config.GetLatestChangesRefreshInterval() * time.Second); err != nil {
					log.Printf("submission cache: refresh failed: %v", err)
				}
			case <-stopRefresh:
				ticker.Stop()
				return
			}
		}
	}()
}

// StopSubmissionCache stops the background ticker.
func StopSubmissionCache() {
	if stopRefresh != nil {
		close(stopRefresh)
	}
}

// GetSubmissionLatestChanges returns only items matching the requested filingId.
func GetSubmissionLatestChanges(filingID string) ([]models.FormDataLegacy, time.Time) {
	submissionChangeCache.mu.RLock()
	defer submissionChangeCache.mu.RUnlock()

	var out []models.FormDataLegacy
	for _, item := range submissionChangeCache.data {
		if item.FilingID == filingID {
			out = append(out, item)
		}
	}
	return out, submissionChangeCache.updatedAt
}

// helpers
func refreshSubmissionCacheWithTimeout(timeout time.Duration) error {
	var ctx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	items, err := fetchLatestChangesFromDB(ctx)
	if err != nil {
		return err
	}

	submissionChangeCache.mu.Lock()
	submissionChangeCache.data = items
	submissionChangeCache.updatedAt = time.Now()
	submissionChangeCache.mu.Unlock()

	log.Printf("submission cache: refreshed with %d items", len(items))
	return nil
}

func fetchLatestChangesFromDB(ctx context.Context) ([]models.FormDataLegacy, error) {
	db := GetDB()
	since := time.Now().Add(-24 * time.Hour) // refresh the cache every 24 hrs

	var results []models.FormDataLegacy
	err := db.WithContext(ctx).
		Model(&models.FormDataLegacy{}).
		Where("createdAt >= ?", since).
		Order("createdAt DESC").
		Find(&results).Error

	return results, err
}
