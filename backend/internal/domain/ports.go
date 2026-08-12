package domain

import (
	"context"
	"time"
)


type DeviceProvider interface {
	
	FetchDevices(ctx context.Context) ([]Device, error)

	Name() string
}


type SnapshotStore interface {
	Get() Snapshot
	Set(Snapshot)
}


type PreferenceRepository interface {
	GetSettings(ctx context.Context, userID string) (UserSettings, error)
	SaveSettings(ctx context.Context, settings UserSettings) error

	ListDevicePreferences(ctx context.Context, userID string) ([]DevicePreference, error)
	GetDevicePreference(ctx context.Context, userID, deviceID string) (DevicePreference, error)
	SaveDevicePreference(ctx context.Context, pref DevicePreference) error
	DeleteDevicePreference(ctx context.Context, userID, deviceID string) error

	Reset(ctx context.Context, userID string) error
}


type IconRepository interface {
	Save(ctx context.Context, icon Icon) (Icon, error)
	Get(ctx context.Context, id string) (Icon, error)
	DeleteForDevice(ctx context.Context, userID, deviceID string) error

	CountForUser(ctx context.Context, userID string) (int64, error)
}


type HistoryRepository interface {
	Append(ctx context.Context, points []HistoryPoint) error
	List(ctx context.Context, deviceID string, since time.Time, limit int) ([]HistoryPoint, error)
	Prune(ctx context.Context, before time.Time) (int64, error)
}


type Publisher interface {
	Publish(event string, payload any)
	ClientCount() int
}


type Clock interface {
	Now() time.Time
}


type SystemClock struct{}


func (SystemClock) Now() time.Time { return time.Now() }
