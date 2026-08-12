package domain

import "time"

// DeviceView is a live device merged with the requesting user's preferences.
// It is the unit the API serves and the UI renders.
type DeviceView struct {
	Device

	// DisplayName is the user's custom name when set, otherwise Device.Name.
	DisplayName string
	// Renamed reports whether DisplayName came from a preference.
	Renamed bool

	Hidden        bool
	Pinned        bool
	MarkerIcon    MarkerIcon
	MarkerColor   string
	CustomIconURL string
	Notes         string
	SortIndex     int
}

// FleetSummary is the aggregate state of the fleet shown in the header KPIs.
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

// StatusFilter narrows a device query to a subset of drive/connection states.
type StatusFilter string

const (
	StatusAll     StatusFilter = "all"
	StatusDriving StatusFilter = "driving"
	StatusIdle    StatusFilter = "idle"
	StatusOff     StatusFilter = "off"
	StatusOnline  StatusFilter = "online"
	StatusOffline StatusFilter = "offline"
)

// DeviceQuery captures the filtering/sorting a caller asked for. Zero values
// mean "use the user's stored preference", which is resolved in the service.
type DeviceQuery struct {
	Search        string
	Status        StatusFilter
	SortKey       SortKey
	SortDirection SortDirection
	IncludeHidden bool
	OnlyPinned    bool
}

// Matches reports whether a device passes the status filter.
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
