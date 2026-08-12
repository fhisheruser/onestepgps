
package dto

import (
	"time"

	"fleetview/internal/domain"
	"fleetview/internal/service"
)


type Position struct {
	Lat        float64    `json:"lat"`
	Lng        float64    `json:"lng"`
	Altitude   float64    `json:"altitude"`
	Heading    float64    `json:"heading"`
	Speed      float64    `json:"speed"`
	SpeedUnit  string     `json:"speedUnit"`
	RecordedAt *time.Time `json:"recordedAt"`
	Valid      bool       `json:"valid"`
}


type DevicePrefs struct {
	Hidden        bool   `json:"hidden"`
	Pinned        bool   `json:"pinned"`
	MarkerIcon    string `json:"markerIcon"`
	MarkerColor   string `json:"markerColor"`
	CustomIconURL string `json:"customIconUrl,omitempty"`
	Notes         string `json:"notes,omitempty"`
	SortIndex     int    `json:"sortIndex"`
}


type Device struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderName string `json:"providerName"`
	Renamed      bool   `json:"renamed"`
	Make         string `json:"make"`
	Model        string `json:"model"`
	FactoryID    string `json:"factoryId,omitempty"`

	Online bool `json:"online"`
	Active bool `json:"active"`

	DriveStatus        string     `json:"driveStatus"`
	DriveStatusSeconds float64    `json:"driveStatusSeconds"`
	DriveStatusSince   *time.Time `json:"driveStatusSince,omitempty"`

	Position Position `json:"position"`

	Odometer     float64  `json:"odometer"`
	OdometerUnit string   `json:"odometerUnit,omitempty"`
	Groups       []string `json:"groups"`

	Preferences DevicePrefs `json:"preferences"`
}


type Summary struct {
	Total       int        `json:"total"`
	Visible     int        `json:"visible"`
	Hidden      int        `json:"hidden"`
	Online      int        `json:"online"`
	Offline     int        `json:"offline"`
	Driving     int        `json:"driving"`
	Idle        int        `json:"idle"`
	Off         int        `json:"off"`
	AvgSpeed    float64    `json:"avgSpeed"`
	SpeedUnit   string     `json:"speedUnit"`
	LastUpdated *time.Time `json:"lastUpdated"`
	Stale       bool       `json:"stale"`
}


type Settings struct {
	Theme              string `json:"theme"`
	SortKey            string `json:"sortKey"`
	SortDirection      string `json:"sortDirection"`
	SpeedUnit          string `json:"speedUnit"`
	MapType            string `json:"mapType"`
	ShowOfflineDevices bool   `json:"showOfflineDevices"`
	ClusterMarkers     bool   `json:"clusterMarkers"`
	ShowTrails         bool   `json:"showTrails"`
	AnimateMarkers     bool   `json:"animateMarkers"`
	AutoFitBounds      bool   `json:"autoFitBounds"`
	RefreshSeconds     int    `json:"refreshSeconds"`
}


type DevicePreference struct {
	DeviceID      string     `json:"deviceId"`
	Hidden        bool       `json:"hidden"`
	DisplayName   string     `json:"displayName,omitempty"`
	MarkerIcon    string     `json:"markerIcon"`
	MarkerColor   string     `json:"markerColor"`
	CustomIconURL string     `json:"customIconUrl,omitempty"`
	Pinned        bool       `json:"pinned"`
	SortIndex     int        `json:"sortIndex"`
	Notes         string     `json:"notes,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}


type Preferences struct {
	Settings Settings           `json:"settings"`
	Devices  []DevicePreference `json:"devices"`
}


type Meta struct {
	FetchedAt  *time.Time `json:"fetchedAt"`
	AgeSeconds float64    `json:"ageSeconds"`
	Stale      bool       `json:"stale"`
	Error      string     `json:"error,omitempty"`
	Count      int        `json:"count"`
	ServerTime time.Time  `json:"serverTime"`
}


type Feed struct {
	Devices  []Device `json:"devices"`
	Summary  Summary  `json:"summary"`
	Settings Settings `json:"settings"`
	Meta     Meta     `json:"meta"`
}


type HistoryPoint struct {
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	Speed       float64   `json:"speed"`
	Heading     float64   `json:"heading"`
	DriveStatus string    `json:"driveStatus"`
	RecordedAt  time.Time `json:"recordedAt"`
}


type RuntimeConfig struct {
	GoogleMapsAPIKey string `json:"googleMapsApiKey"`
	GoogleMapsMapID  string `json:"googleMapsMapId"`
	RefreshSeconds   int    `json:"refreshSeconds"`
	RealtimeEnabled  bool   `json:"realtimeEnabled"`
	DemoMode         bool   `json:"demoMode"`
	Provider         string `json:"provider"`
	Version          string `json:"version"`
	MaxIconBytes     int64  `json:"maxIconBytes"`
}


type Envelope struct {
	Error *APIError `json:"error,omitempty"`
}


type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}


func FromDeviceView(v domain.DeviceView) Device {
	groups := v.Groups
	if groups == nil {
		groups = []string{}
	}
	return Device{
		ID:                 v.ID,
		Name:               v.DisplayName,
		ProviderName:       v.Device.Name,
		Renamed:            v.Renamed,
		Make:               v.Make,
		Model:              v.Model,
		FactoryID:          v.FactoryID,
		Online:             v.Online,
		Active:             v.Active,
		DriveStatus:        string(v.DriveStatus),
		DriveStatusSeconds: v.DriveStatusDuration.Seconds(),
		DriveStatusSince:   timePtr(v.DriveStatusSince),
		Position: Position{
			Lat:        v.Position.Lat,
			Lng:        v.Position.Lng,
			Altitude:   v.Position.Altitude,
			Heading:    v.Position.Heading,
			Speed:      v.Position.Speed,
			SpeedUnit:  v.Position.SpeedUnit,
			RecordedAt: timePtr(v.Position.RecordedAt),
			Valid:      v.Position.Valid(),
		},
		Odometer:     v.Odometer,
		OdometerUnit: v.OdometerUnit,
		Groups:       groups,
		Preferences: DevicePrefs{
			Hidden:        v.Hidden,
			Pinned:        v.Pinned,
			MarkerIcon:    string(v.MarkerIcon),
			MarkerColor:   v.MarkerColor,
			CustomIconURL: v.CustomIconURL,
			Notes:         v.Notes,
			SortIndex:     v.SortIndex,
		},
	}
}


func FromFeed(feed service.Feed, now time.Time) Feed {
	devices := make([]Device, 0, len(feed.Devices))
	for _, v := range feed.Devices {
		devices = append(devices, FromDeviceView(v))
	}

	return Feed{
		Devices:  devices,
		Summary:  FromSummary(feed.Summary),
		Settings: FromSettings(feed.Settings),
		Meta: Meta{
			FetchedAt:  timePtr(feed.Snapshot.FetchedAt),
			AgeSeconds: feed.Age.Seconds(),
			Stale:      feed.Snapshot.Stale,
			Error:      feed.Snapshot.Error,
			Count:      len(devices),
			ServerTime: now.UTC(),
		},
	}
}


func FromSummary(s domain.FleetSummary) Summary {
	return Summary{
		Total:       s.Total,
		Visible:     s.Visible,
		Hidden:      s.Hidden,
		Online:      s.Online,
		Offline:     s.Offline,
		Driving:     s.Driving,
		Idle:        s.Idle,
		Off:         s.Off,
		AvgSpeed:    s.AvgSpeed,
		SpeedUnit:   s.SpeedUnit,
		LastUpdated: timePtr(s.LastUpdated),
		Stale:       s.Stale,
	}
}


func FromSettings(s domain.UserSettings) Settings {
	return Settings{
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


func FromDevicePreference(p domain.DevicePreference) DevicePreference {
	return DevicePreference{
		DeviceID:      p.DeviceID,
		Hidden:        p.Hidden,
		DisplayName:   p.DisplayName,
		MarkerIcon:    string(p.MarkerIcon),
		MarkerColor:   p.MarkerColor,
		CustomIconURL: p.CustomIconURL,
		Pinned:        p.Pinned,
		SortIndex:     p.SortIndex,
		Notes:         p.Notes,
		UpdatedAt:     timePtr(p.UpdatedAt),
	}
}


func FromPreferences(p service.Preferences) Preferences {
	devices := make([]DevicePreference, 0, len(p.Devices))
	for _, d := range p.Devices {
		devices = append(devices, FromDevicePreference(d))
	}
	return Preferences{Settings: FromSettings(p.Settings), Devices: devices}
}


func FromHistory(points []domain.HistoryPoint) []HistoryPoint {
	out := make([]HistoryPoint, 0, len(points))
	for _, p := range points {
		out = append(out, HistoryPoint{
			Lat:         p.Lat,
			Lng:         p.Lng,
			Speed:       p.Speed,
			Heading:     p.Heading,
			DriveStatus: string(p.DriveStatus),
			RecordedAt:  p.RecordedAt.UTC(),
		})
	}
	return out
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}
