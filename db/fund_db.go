package db

import (
	"fmt"
	"form_pf_backend/models"
	"sync"
)

// Funds are loaded once at startup, the same way form question metadata is.
var (
	staticFundList []models.Fund
	staticFundMap  map[string]models.Fund
	staticFundMu   sync.RWMutex
)

// InitFundMap loads every fund into memory. Fund rows do not change per request.
func InitFundMap() error {
	var rows []models.Fund
	if err := GetDB().Find(&rows).Error; err != nil {
		return fmt.Errorf("failed to load funds: %w", err)
	}
	list := make([]models.Fund, len(rows))
	copy(list, rows)
	byID := make(map[string]models.Fund, len(rows))
	for _, fund := range rows {
		byID[fund.FundID] = fund
	}

	staticFundMu.Lock()
	staticFundList = list
	staticFundMap = byID
	staticFundMu.Unlock()
	return nil
}

func loadedFunds() (map[string]models.Fund, error) {
	staticFundMu.RLock()
	byID := staticFundMap
	staticFundMu.RUnlock()
	if byID != nil {
		return byID, nil
	}
	if err := InitFundMap(); err != nil {
		return nil, err
	}
	staticFundMu.RLock()
	defer staticFundMu.RUnlock()
	return staticFundMap, nil
}

func GetAllFunds() ([]models.Fund, error) {
	if _, err := loadedFunds(); err != nil {
		return nil, err
	}
	staticFundMu.RLock()
	defer staticFundMu.RUnlock()
	out := make([]models.Fund, len(staticFundList))
	copy(out, staticFundList)
	return out, nil
}

// GetFundMap returns the preloaded name and type for each requested fund id.
func GetFundMap(fundSeriesIDs []string) (map[string]models.Fund, error) {
	if len(fundSeriesIDs) == 0 {
		return map[string]models.Fund{}, nil
	}
	byID, err := loadedFunds()
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.Fund, len(fundSeriesIDs))
	for _, id := range fundSeriesIDs {
		if fund, ok := byID[id]; ok {
			out[id] = fund
		}
	}
	return out, nil
}
