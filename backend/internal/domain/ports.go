package domain

import (
	"context"
	"time"
)

// DeviceProvider is the outbound port for a live GPS data source. The
// OneStepGPS HTTP client and the demo simulator both satisfy it, which is what
// lets the application run with or without upstream credentials.
type DeviceProvider interface {
	// FetchDevices returns the current state of every device visible to the
	// configured account. Implementations must respect ctx cancellation.
	FetchDevices(ctx context.Context) ([]Device, error)
	// Name identifies the provider in logs and health output.
	Name() string
}

// SnapshotStore is the outbound port for the in-memory fleet cache.
type SnapshotStore interface {
	Get() Snapshot
	Set(Snapshot)
}

// PreferenceRepository persists per-user personalisation.
type PreferenceRepository interface {
	GetSettings(ctx context.Context, userID string) (UserSettings, error)
	SaveSettings(ctx context.Context, settings UserSettings) error

	ListDevicePreferences(ctx context.Context, userID string) ([]DevicePreference, error)
	GetDevicePreference(ctx context.Context, userID, deviceID string) (DevicePreference, error)
	SaveDevicePreference(ctx context.Context, pref DevicePreference) error
	DeleteDevicePreference(ctx context.Context, userID, deviceID string) error

	Reset(ctx context.Context, userID string) error
}

// IconRepository stores user-uploaded marker images.
type IconRepository interface {
	Save(ctx context.Context, icon Icon) (Icon, error)
	Get(ctx context.Context, id string) (Icon, error)
	DeleteForDevice(ctx context.Context, userID, deviceID string) error
	// CountForUser backs the per-user upload quota.
	CountForUser(ctx context.Context, userID string) (int64, error)
}

// HistoryRepository persists device breadcrumbs for trail rendering.
type HistoryRepository interface {
	Append(ctx context.Context, points []HistoryPoint) error
	List(ctx context.Context, deviceID string, since time.Time, limit int) ([]HistoryPoint, error)
	Prune(ctx context.Context, before time.Time) (int64, error)
}

// Publisher pushes fleet updates to connected realtime clients. The poller
// depends on this narrow interface rather than on the WebSocket hub itself.
type Publisher interface {
	Publish(event string, payload any)
	ClientCount() int
}

// Clock is injectable time, so tests do not need to sleep.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }
