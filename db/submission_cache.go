package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"form_pf_backend/config"
	"form_pf_backend/models"
)

type submissionCache struct {
	mu        sync.RWMutex
	data      []models.EditorCache
	updatedAt time.Time
}

var (
	submissionChangeCache *submissionCache
	stopRefresh           chan struct{}
)

// InitSubmissionCache starts the background refresher.
func InitSubmissionCache(interval time.Duration) {
	submissionChangeCache = &submissionCache{
		data: make([]models.EditorCache, 0),
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
func GetSubmissionLatestChanges(filingID, userId string) ([]models.EditorCache, time.Time) {
	submissionChangeCache.mu.RLock()
	defer submissionChangeCache.mu.RUnlock()

	var out []models.EditorCache
	for _, item := range submissionChangeCache.data {
		fmt.Println(userId)
		if item.FilingID == filingID && item.CreatedBy != userId {
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

// func fetchLatestChangesFromDB(ctx context.Context) ([]models.EditorCache, error) {
// 	db := GetDB()
// 	since := time.Now().Add(-24 * time.Hour) // refresh the cache every 24 hrs

// 	var results []models.EditorCache
// 	err := db.WithContext(ctx).
// 		Model(&models.EditorCache{}).
// 		Where("createdAt >= ?", since).
// 		Order("createdAt DESC").
// 		Find(&results).Error

// 	return results, err
// }

func fetchLatestChangesFromDB(ctx context.Context) ([]models.EditorCache, error) {
	db := GetDB()
	since := time.Now().Add(-24 * time.Hour)

	var results []models.EditorCache
	err := db.WithContext(ctx).
		Model(&models.FormDataLegacy{}).
		Select(
			"filingId",
			"fundSeriesId",
			"fieldNo",
			"formLineNo",
			"value",
			"form_data_legacy.createdBy",
			"form_data_legacy.createdAt",
			"users.userName",
		).
		Joins("LEFT JOIN users ON user.id = form_data_legacy.createdBy").
		Where("form_data_legacy.createdAt >= ?", since).
		Order("form_data_legacy.createdAt DESC").
		Scan(&results).Error

	return results, err
}
