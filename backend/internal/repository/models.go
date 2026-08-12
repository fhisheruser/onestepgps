package repository

import (
	"time"

	"fleetview/internal/domain"
)

// The records below are persistence models, deliberately separate from the
// domain entities. Storage concerns (column sizes, indexes, timestamps) stay
// here, and the domain stays free of GORM tags.

// Note on the deliberate absence of `default:` tags below: GORM treats a
// column with a default as "omit the Go zero value on insert so the database
// can fill it in". For a boolean preference that silently turns `false` into
// the default `true`. Defaults belong to domain.DefaultUserSettings, which is
// applied on a read miss, so every column here is always written explicitly.
type userSettingsRecord struct {
	UserID             string `gorm:"primaryKey;size:64"`
	Theme              string `gorm:"size:16;not null"`
	SortKey            string `gorm:"size:16;not null"`
	SortDirection      string `gorm:"size:4;not null"`
	SpeedUnit          string `gorm:"size:8;not null"`
	MapType            string `gorm:"size:16;not null"`
	ShowOfflineDevices bool   `gorm:"not null"`
	ClusterMarkers     bool   `gorm:"not null"`
	ShowTrails         bool   `gorm:"not null"`
	AnimateMarkers     bool   `gorm:"not null"`
	AutoFitBounds      bool   `gorm:"not null"`
	RefreshSeconds     int    `gorm:"not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (userSettingsRecord) TableName() string { return "user_settings" }

func (r userSettingsRecord) toDomain() domain.UserSettings {
	return domain.UserSettings{
		UserID:             r.UserID,
		Theme:              domain.Theme(r.Theme),
		SortKey:            domain.SortKey(r.SortKey),
		SortDirection:      domain.SortDirection(r.SortDirection),
		SpeedUnit:          domain.SpeedUnit(r.SpeedUnit),
		MapType:            r.MapType,
		ShowOfflineDevices: r.ShowOfflineDevices,
		ClusterMarkers:     r.ClusterMarkers,
		ShowTrails:         r.ShowTrails,
		AnimateMarkers:     r.AnimateMarkers,
		AutoFitBounds:      r.AutoFitBounds,
		RefreshSeconds:     r.RefreshSeconds,
		UpdatedAt:          r.UpdatedAt,
	}
}

func settingsRecordFrom(s domain.UserSettings) userSettingsRecord {
	return userSettingsRecord{
		UserID:             s.UserID,
		Theme:              string(s.Theme),
		SortKey:            string(s.SortKey),
		SortDirection:      string(s.SortDirection),
		SpeedUnit:          string(s.SpeedUnit),
		MapType:            s.MapType,
		ShowOfflineDevices: s.ShowOfflineDevices,
		ClusterMarkers:     s.ClusterMarkers,
		ShowTrails:         s.ShowTrails,
		AnimateMarkers:     s.AnimateMarkers,
		AutoFitBounds:      s.AutoFitBounds,
		RefreshSeconds:     s.RefreshSeconds,
	}
}

type devicePreferenceRecord struct {
	ID            uint   `gorm:"primaryKey;autoIncrement"`
	UserID        string `gorm:"size:64;not null;uniqueIndex:ux_pref_user_device,priority:1"`
	DeviceID      string `gorm:"size:128;not null;uniqueIndex:ux_pref_user_device,priority:2"`
	Hidden        bool   `gorm:"not null"`
	DisplayName   string `gorm:"size:64"`
	MarkerIcon    string `gorm:"size:16"`
	MarkerColor   string `gorm:"size:9"`
	CustomIconURL string `gorm:"size:255"`
	Pinned        bool   `gorm:"not null"`
	SortIndex     int    `gorm:"not null"`
	Notes         string `gorm:"size:500"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (devicePreferenceRecord) TableName() string { return "device_preferences" }

func (r devicePreferenceRecord) toDomain() domain.DevicePreference {
	return domain.DevicePreference{
		UserID:        r.UserID,
		DeviceID:      r.DeviceID,
		Hidden:        r.Hidden,
		DisplayName:   r.DisplayName,
		MarkerIcon:    domain.MarkerIcon(r.MarkerIcon),
		MarkerColor:   r.MarkerColor,
		CustomIconURL: r.CustomIconURL,
		Pinned:        r.Pinned,
		SortIndex:     r.SortIndex,
		Notes:         r.Notes,
		UpdatedAt:     r.UpdatedAt,
	}
}

func preferenceRecordFrom(p domain.DevicePreference) devicePreferenceRecord {
	return devicePreferenceRecord{
		UserID:        p.UserID,
		DeviceID:      p.DeviceID,
		Hidden:        p.Hidden,
		DisplayName:   p.DisplayName,
		MarkerIcon:    string(p.MarkerIcon),
		MarkerColor:   p.MarkerColor,
		CustomIconURL: p.CustomIconURL,
		Pinned:        p.Pinned,
		SortIndex:     p.SortIndex,
		Notes:         p.Notes,
	}
}

type iconRecord struct {
	ID          string `gorm:"primaryKey;size:32"`
	UserID      string `gorm:"size:64;not null;index:ix_icon_user_device,priority:1"`
	DeviceID    string `gorm:"size:128;not null;index:ix_icon_user_device,priority:2"`
	ContentType string `gorm:"size:64;not null"`
	Size        int    `gorm:"not null"`
	Data        []byte `gorm:"not null"`
	CreatedAt   time.Time
}

func (iconRecord) TableName() string { return "device_icons" }

func (r iconRecord) toDomain() domain.Icon {
	return domain.Icon{
		ID:          r.ID,
		UserID:      r.UserID,
		DeviceID:    r.DeviceID,
		ContentType: r.ContentType,
		Size:        r.Size,
		Data:        r.Data,
		CreatedAt:   r.CreatedAt,
	}
}

type historyPointRecord struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	DeviceID    string    `gorm:"size:128;not null;index:ix_history_device_time,priority:1"`
	Lat         float64   `gorm:"not null"`
	Lng         float64   `gorm:"not null"`
	Speed       float64   `gorm:"not null"`
	Heading     float64   `gorm:"not null"`
	DriveStatus string    `gorm:"size:16"`
	RecordedAt  time.Time `gorm:"not null;index:ix_history_device_time,priority:2"`
}

func (historyPointRecord) TableName() string { return "device_history" }

func (r historyPointRecord) toDomain() domain.HistoryPoint {
	return domain.HistoryPoint{
		ID:          r.ID,
		DeviceID:    r.DeviceID,
		Lat:         r.Lat,
		Lng:         r.Lng,
		Speed:       r.Speed,
		Heading:     r.Heading,
		DriveStatus: domain.DriveStatus(r.DriveStatus),
		RecordedAt:  r.RecordedAt,
	}
}

func historyRecordFrom(p domain.HistoryPoint) historyPointRecord {
	recordedAt := p.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = time.Now()
	}
	return historyPointRecord{
		DeviceID:    p.DeviceID,
		Lat:         p.Lat,
		Lng:         p.Lng,
		Speed:       p.Speed,
		Heading:     p.Heading,
		DriveStatus: string(p.DriveStatus),
		RecordedAt:  recordedAt.UTC(),
	}
}
