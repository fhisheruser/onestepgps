package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"fleetview/internal/domain"
)

// historyBatchSize bounds how many rows go into a single INSERT.
const historyBatchSize = 100

// HistoryRepository persists device breadcrumbs used to draw trails.
type HistoryRepository struct {
	db *gorm.DB
}

// NewHistoryRepository builds the repository.
func NewHistoryRepository(db *gorm.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

var _ domain.HistoryRepository = (*HistoryRepository)(nil)

// Append stores a batch of breadcrumbs.
func (r *HistoryRepository) Append(ctx context.Context, points []domain.HistoryPoint) error {
	if len(points) == 0 {
		return nil
	}
	records := make([]historyPointRecord, 0, len(points))
	for _, p := range points {
		if p.DeviceID == "" {
			continue
		}
		records = append(records, historyRecordFrom(p))
	}
	if len(records) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(&records, historyBatchSize).Error; err != nil {
		return fmt.Errorf("insert history points: %w", err)
	}
	return nil
}

// List returns up to limit breadcrumbs for a device since a point in time,
// oldest first (the order a polyline wants). The newest points are kept when
// the window contains more than limit rows.
func (r *HistoryRepository) List(ctx context.Context, deviceID string, since time.Time, limit int) ([]domain.HistoryPoint, error) {
	if limit <= 0 {
		limit = 500
	}
	var records []historyPointRecord
	err := r.db.WithContext(ctx).
		Where("device_id = ? AND recorded_at >= ?", deviceID, since.UTC()).
		Order("recorded_at desc").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}

	points := make([]domain.HistoryPoint, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		points = append(points, records[i].toDomain())
	}
	return points, nil
}

// Prune deletes breadcrumbs older than the cutoff and reports how many rows
// were removed.
func (r *HistoryRepository) Prune(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("recorded_at < ?", before.UTC()).
		Delete(&historyPointRecord{})
	if result.Error != nil {
		return 0, fmt.Errorf("prune history: %w", result.Error)
	}
	return result.RowsAffected, nil
}
