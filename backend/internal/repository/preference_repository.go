package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"fleetview/internal/domain"
)


type PreferenceRepository struct {
	db *gorm.DB
}


func NewPreferenceRepository(db *gorm.DB) *PreferenceRepository {
	return &PreferenceRepository{db: db}
}

var _ domain.PreferenceRepository = (*PreferenceRepository)(nil)


func (r *PreferenceRepository) GetSettings(ctx context.Context, userID string) (domain.UserSettings, error) {
	var record userSettingsRecord
	err := r.db.WithContext(ctx).First(&record, "user_id = ?", userID).Error

	switch {
	case err == nil:
		settings := record.toDomain()
		settings.Normalize()
		return settings, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.DefaultUserSettings(userID), nil
	default:
		return domain.UserSettings{}, fmt.Errorf("query user settings: %w", err)
	}
}


func (r *PreferenceRepository) SaveSettings(ctx context.Context, settings domain.UserSettings) error {
	if settings.UserID == "" {
		return domain.NewValidationError("userId", "user id is required")
	}
	settings.Normalize()
	record := settingsRecordFrom(settings)

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"theme", "sort_key", "sort_direction", "speed_unit", "map_type",
			"show_offline_devices", "cluster_markers", "show_trails",
			"animate_markers", "auto_fit_bounds", "refresh_seconds", "updated_at",
		}),
	}).Create(&record).Error
	if err != nil {
		return fmt.Errorf("save user settings: %w", err)
	}
	return nil
}


func (r *PreferenceRepository) ListDevicePreferences(ctx context.Context, userID string) ([]domain.DevicePreference, error) {
	var records []devicePreferenceRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("sort_index asc, device_id asc").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query device preferences: %w", err)
	}

	prefs := make([]domain.DevicePreference, 0, len(records))
	for _, record := range records {
		prefs = append(prefs, record.toDomain())
	}
	return prefs, nil
}


func (r *PreferenceRepository) GetDevicePreference(ctx context.Context, userID, deviceID string) (domain.DevicePreference, error) {
	var record devicePreferenceRecord
	err := r.db.WithContext(ctx).First(&record, "user_id = ? AND device_id = ?", userID, deviceID).Error

	switch {
	case err == nil:
		return record.toDomain(), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.DevicePreference{UserID: userID, DeviceID: deviceID}, nil
	default:
		return domain.DevicePreference{}, fmt.Errorf("query device preference: %w", err)
	}
}


func (r *PreferenceRepository) SaveDevicePreference(ctx context.Context, pref domain.DevicePreference) error {
	if pref.UserID == "" || pref.DeviceID == "" {
		return domain.NewValidationError("deviceId", "user id and device id are required")
	}
	if pref.IsZeroValue() {
		return r.DeleteDevicePreference(ctx, pref.UserID, pref.DeviceID)
	}

	record := preferenceRecordFrom(pref)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"hidden", "display_name", "marker_icon", "marker_color",
			"custom_icon_url", "pinned", "sort_index", "notes", "updated_at",
		}),
	}).Create(&record).Error
	if err != nil {
		return fmt.Errorf("save device preference: %w", err)
	}
	return nil
}


func (r *PreferenceRepository) DeleteDevicePreference(ctx context.Context, userID, deviceID string) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Delete(&devicePreferenceRecord{}).Error
	if err != nil {
		return fmt.Errorf("delete device preference: %w", err)
	}
	return nil
}


func (r *PreferenceRepository) Reset(ctx context.Context, userID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&devicePreferenceRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&iconRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&userSettingsRecord{}).Error
	})
	if err != nil {
		return fmt.Errorf("reset preferences: %w", err)
	}
	return nil
}
