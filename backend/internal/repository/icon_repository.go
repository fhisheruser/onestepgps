package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"fleetview/internal/domain"
)


type IconRepository struct {
	db *gorm.DB
}


func NewIconRepository(db *gorm.DB) *IconRepository {
	return &IconRepository{db: db}
}

var _ domain.IconRepository = (*IconRepository)(nil)

func (r *IconRepository) Save(ctx context.Context, icon domain.Icon) (domain.Icon, error) {
	record := iconRecord{
		ID:          icon.ID,
		UserID:      icon.UserID,
		DeviceID:    icon.DeviceID,
		ContentType: icon.ContentType,
		Size:        icon.Size,
		Data:        icon.Data,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return domain.Icon{}, fmt.Errorf("insert icon: %w", err)
	}
	return record.toDomain(), nil
}


func (r *IconRepository) Get(ctx context.Context, id string) (domain.Icon, error) {
	var record iconRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error

	switch {
	case err == nil:
		return record.toDomain(), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.Icon{}, fmt.Errorf("icon %q: %w", id, domain.ErrNotFound)
	default:
		return domain.Icon{}, fmt.Errorf("query icon: %w", err)
	}
}


func (r *IconRepository) CountForUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&iconRecord{}).
		Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count icons: %w", err)
	}
	return count, nil
}

func (r *IconRepository) DeleteForDevice(ctx context.Context, userID, deviceID string) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Delete(&iconRecord{}).Error
	if err != nil {
		return fmt.Errorf("delete icons: %w", err)
	}
	return nil
}
