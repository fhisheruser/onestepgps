
package domain

import (
	"math"
	"strings"
	"time"
)


type DriveStatus string

const (
	DriveStatusDriving DriveStatus = "driving"
	DriveStatusIdle    DriveStatus = "idle"
	DriveStatusOff     DriveStatus = "off"
	DriveStatusUnknown DriveStatus = "unknown"
)


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

type Position struct {
	Lat float64
	Lng float64
	
	Altitude float64
	
	Heading float64

	Speed     float64
	SpeedUnit string

	RecordedAt time.Time
}


func (p Position) Valid() bool {
	if math.IsNaN(p.Lat) || math.IsNaN(p.Lng) {
		return false
	}
	if p.Lat < -90 || p.Lat > 90 || p.Lng < -180 || p.Lng > 180 {
		return false
	}
	return math.Abs(p.Lat) > 1e-6 || math.Abs(p.Lng) > 1e-6
}


type Device struct {
	ID        string
	Name      string
	Make      string
	Model     string
	FactoryID string


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


func (d Device) Moving() bool { return d.DriveStatus == DriveStatusDriving }


type Snapshot struct {
	Devices   []Device
	FetchedAt time.Time
	
	Stale bool
	
	Error string
}


func (s Snapshot) Age(now time.Time) time.Duration {
	if s.FetchedAt.IsZero() {
		return 0
	}
	return now.Sub(s.FetchedAt)
}


func (s Snapshot) Empty() bool { return s.FetchedAt.IsZero() }


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
