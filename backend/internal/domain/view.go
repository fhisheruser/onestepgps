package domain

import "time"


type DeviceView struct {
	Device

	DisplayName string
	
	Renamed bool

	Hidden        bool
	Pinned        bool
	MarkerIcon    MarkerIcon
	MarkerColor   string
	CustomIconURL string
	Notes         string
	SortIndex     int
}


type FleetSummary struct {
	Total       int
	Visible     int
	Hidden      int
	Online      int
	Offline     int
	Driving     int
	Idle        int
	Off         int
	AvgSpeed    float64
	SpeedUnit   string
	LastUpdated time.Time
	Stale       bool
}


type StatusFilter string

const (
	StatusAll     StatusFilter = "all"
	StatusDriving StatusFilter = "driving"
	StatusIdle    StatusFilter = "idle"
	StatusOff     StatusFilter = "off"
	StatusOnline  StatusFilter = "online"
	StatusOffline StatusFilter = "offline"
)


type DeviceQuery struct {
	Search        string
	Status        StatusFilter
	SortKey       SortKey
	SortDirection SortDirection
	IncludeHidden bool
	OnlyPinned    bool
}

func (q DeviceQuery) Matches(d DeviceView) bool {
	switch q.Status {
	case StatusDriving:
		return d.DriveStatus == DriveStatusDriving
	case StatusIdle:
		return d.DriveStatus == DriveStatusIdle
	case StatusOff:
		return d.DriveStatus == DriveStatusOff
	case StatusOnline:
		return d.Online
	case StatusOffline:
		return !d.Online
	default:
		return true
	}
}
