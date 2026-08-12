// Package domain contains the enterprise entities and the port interfaces of
// the application. It has no knowledge of HTTP, GORM, SQLite or any other
// delivery/infrastructure concern: every other package may depend on domain,
// domain depends on nothing but the standard library.
package domain

import (
	"math"
	"strings"
	"time"
)

// DriveStatus is the normalised movement state of a vehicle. The upstream API
// uses a free-form string, so it is mapped onto this closed set to keep the
// rest of the system (and the UI) free of provider-specific vocabulary.
type DriveStatus string

const (
	DriveStatusDriving DriveStatus = "driving"
	DriveStatusIdle    DriveStatus = "idle"
	DriveStatusOff     DriveStatus = "off"
	DriveStatusUnknown DriveStatus = "unknown"
)

// ParseDriveStatus maps a provider drive-status string onto a DriveStatus.
// Unrecognised, empty or null values degrade to DriveStatusUnknown rather than
// failing the whole fetch: one odd device must never blank out the fleet.
func ParseDriveStatus(raw string) DriveStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "driving", "drive", "moving", "in_motion":
		return DriveStatusDriving
	case "idle", "idling":
		return DriveStatusIdle
	case "off", "stopped", "parked", "stop":
		return DriveStatusOff
	default:
		return DriveStatusUnknown
	}
}

// Position is a single GPS fix.
type Position struct {
	Lat float64
	Lng float64
	// Altitude in metres above sea level.
	Altitude float64
	// Heading in degrees clockwise from true north (0-359).
	Heading float64
	// Speed expressed in SpeedUnit. The provider unit is preserved verbatim so
	// that unit conversion stays a presentation concern driven by user
	// preference instead of being baked into stored data.
	Speed     float64
	SpeedUnit string
	// RecordedAt is the timestamp reported by the tracker (dt_tracker).
	RecordedAt time.Time
}

// Valid reports whether the fix carries usable coordinates. Trackers that have
// never obtained a lock report (0,0), which is in the Gulf of Guinea rather
// than a real vehicle location.
func (p Position) Valid() bool {
	if math.IsNaN(p.Lat) || math.IsNaN(p.Lng) {
		return false
	}
	if p.Lat < -90 || p.Lat > 90 || p.Lng < -180 || p.Lng > 180 {
		return false
	}
	return math.Abs(p.Lat) > 1e-6 || math.Abs(p.Lng) > 1e-6
}

// Device is a tracked asset as the application understands it, independent of
// the shape the upstream provider happens to return.
type Device struct {
	ID        string
	Name      string
	Make      string
	Model     string
	FactoryID string

	// Online is the provider's connectivity flag; Active is derived from
	// active_state and indicates whether the subscription is live.
	Online bool
	Active bool

	DriveStatus         DriveStatus
	DriveStatusDuration time.Duration
	DriveStatusSince    time.Time

	Position Position

	Odometer     float64
	OdometerUnit string

	Groups []string
}

// Moving reports whether the device is currently driving.
func (d Device) Moving() bool { return d.DriveStatus == DriveStatusDriving }

// Snapshot is an immutable, point-in-time view of the whole fleet as produced
// by the background poller. Handlers read snapshots; only the poller writes
// them, which keeps read paths lock-light and always consistent.
type Snapshot struct {
	Devices   []Device
	FetchedAt time.Time
	// Stale is set when the most recent poll failed and the devices below are
	// therefore the last known good data rather than fresh data.
	Stale bool
	// Error carries a human-readable description of the last poll failure.
	Error string
}

// Age returns how long ago the snapshot was fetched.
func (s Snapshot) Age(now time.Time) time.Duration {
	if s.FetchedAt.IsZero() {
		return 0
	}
	return now.Sub(s.FetchedAt)
}

// Empty reports whether the snapshot has never been populated.
func (s Snapshot) Empty() bool { return s.FetchedAt.IsZero() }

// HistoryPoint is one persisted breadcrumb of a device's trail.
type HistoryPoint struct {
	ID          int64
	DeviceID    string
	Lat         float64
	Lng         float64
	Speed       float64
	Heading     float64
	DriveStatus DriveStatus
	RecordedAt  time.Time
}
